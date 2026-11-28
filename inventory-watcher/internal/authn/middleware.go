package authn

import (
	"context"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const claimsKey contextKey = "jwt_claims"

// ClaimsFromContext returns the JWT claims from the request context.
func ClaimsFromContext(ctx context.Context) (jwt.MapClaims, bool) {
	claims, ok := ctx.Value(claimsKey).(jwt.MapClaims)
	return claims, ok
}

// Middleware validates JWT bearer tokens against a JWKS endpoint.
// Compatible with OSAC's authentication model.
type Middleware struct {
	issuerURL           string
	jwksURL             string
	trustIdentityHeader bool
	exportSharedSecret  string
	keys                *sync.Map
	caPool              *x509.CertPool
	logger              *slog.Logger
	client              *http.Client
}

// SetExportSharedSecret configures an optional service-to-service shared secret
// accepted strictly for read-only GET /api/v1/export/* requests (D12).
func (m *Middleware) SetExportSharedSecret(secret string) {
	m.exportSharedSecret = strings.TrimSpace(secret)
	if m.exportSharedSecret != "" && m.logger != nil {
		m.logger.Info("export shared-secret authentication enabled for GET /api/v1/export/*")
	}
}

// New creates a JWT authentication middleware. If issuerURL is empty,
// authentication is disabled (all requests pass through).
func New(issuerURL, caCertPath string, trustIdentityHeader bool, logger *slog.Logger) (*Middleware, error) {
	if issuerURL == "" {
		logger.Warn("authentication disabled — no issuer URL configured")
		return &Middleware{
			trustIdentityHeader: trustIdentityHeader,
			logger:              logger,
		}, nil
	}

	m := &Middleware{
		issuerURL:           issuerURL,
		trustIdentityHeader: trustIdentityHeader,
		keys:                &sync.Map{},
		logger:              logger,
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	if caCertPath != "" {
		caCert, err := os.ReadFile(caCertPath)
		if err != nil {
			return nil, fmt.Errorf("reading CA cert: %w", err)
		}
		pool := x509.NewCertPool()
		pool.AppendCertsFromPEM(caCert)
		m.caPool = pool
		transport.TLSClientConfig = &tls.Config{RootCAs: pool}
	}

	m.client = &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
	}

	jwksURL, err := m.discoverJWKS()
	if err != nil {
		return nil, fmt.Errorf("OIDC discovery failed: %w", err)
	}
	m.jwksURL = jwksURL

	if err := m.refreshKeys(); err != nil {
		return nil, fmt.Errorf("initial JWKS fetch failed: %w", err)
	}

	logger.Info("JWT authentication enabled", "issuer", issuerURL, "jwks", jwksURL, "trust_identity_header", trustIdentityHeader)
	return m, nil
}

// Wrap returns an HTTP handler that validates JWT tokens before calling next.
func (m *Middleware) Wrap(next http.Handler) http.Handler {
	if m.issuerURL == "" {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if m.exportSharedSecret != "" && isExportReadPath(r) {
				if m.matchExportSharedSecret(r) {
					ctx := WithIdentity(r.Context(), exportServiceIdentity())
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
				if m.trustIdentityHeader {
					if xRhHeader := r.Header.Get(HeaderXRhIdentity); xRhHeader != "" {
						id, err := ParseXRhIdentity(xRhHeader)
						if err != nil {
							writeAuthError(w, "invalid X-Rh-Identity: "+err.Error())
							return
						}
						next.ServeHTTP(w, r.WithContext(WithIdentity(r.Context(), id)))
						return
					}
				}
				writeAuthError(w, "missing or invalid export credentials")
				return
			}
			if m.trustIdentityHeader {
				if xRhHeader := r.Header.Get(HeaderXRhIdentity); xRhHeader != "" {
					if id, err := ParseXRhIdentity(xRhHeader); err == nil {
						r = r.WithContext(WithIdentity(r.Context(), id))
					}
				}
			}
			next.ServeHTTP(w, r)
		})
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isBypassPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		// Service-to-service shared secret strictly scoped to read-only GET /api/v1/export/* (D12).
		if m.matchExportSharedSecret(r) {
			ctx := WithIdentity(r.Context(), exportServiceIdentity())
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		// Only trust X-Rh-Identity if explicitly configured (e.g. behind trusted gateway).
		// Note: The trusted gateway must strip any client-supplied X-Rh-Identity headers before injecting its own.
		if m.trustIdentityHeader {
			if xRhHeader := r.Header.Get(HeaderXRhIdentity); xRhHeader != "" {
				identity, err := ParseXRhIdentity(xRhHeader)
				if err != nil {
					m.logger.Warn("invalid X-Rh-Identity header", "error", err)
					writeAuthError(w, "invalid X-Rh-Identity: "+err.Error())
					return
				}
				ctx := r.Context()
				// O2 (D13): If the trusted gateway also forwards the validated Bearer JWT
				// (via Authorization or X-Bearer-Token), verify it via JWKS and enrich
				// IsPlatformAdmin from signed realm_access/is_platform_admin claims.
				if bearer := extractForwardedBearer(r); bearer != "" {
					if claims, jwtErr := m.validateToken(bearer); jwtErr == nil {
						jwtIdentity := IdentityFromJWTClaims(claims)
						if jwtIdentity.IsPlatformAdmin {
							identity.IsPlatformAdmin = true
						}
						ctx = context.WithValue(ctx, claimsKey, claims)
					} else {
						m.logger.Warn("forwarded bearer token validation failed during X-Rh-Identity enrichment", "error", jwtErr)
					}
				}
				ctx = WithIdentity(ctx, identity)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			writeAuthError(w, "missing authentication credentials")
			return
		}

		bearer, ok := strings.CutPrefix(authHeader, "Bearer ")
		if !ok {
			writeAuthError(w, "Authorization header must use Bearer scheme")
			return
		}

		claims, err := m.validateToken(bearer)
		if err != nil {
			m.logger.Warn("token validation failed", "error", err)
			writeAuthError(w, "invalid token: "+err.Error())
			return
		}

		identity := IdentityFromJWTClaims(claims)
		if !identity.IsPlatformAdmin && identity.OrgID == "" {
			m.logger.Warn("token missing org_id claim")
			writeAuthError(w, "invalid token: missing org_id claim")
			return
		}

		ctx := WithIdentity(r.Context(), identity)
		ctx = context.WithValue(ctx, claimsKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func isExportReadPath(r *http.Request) bool {
	return r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v1/export/")
}

func exportServiceIdentity() *Identity {
	return &Identity{
		Username:        "koku-export-service",
		IsPlatformAdmin: true,
		RawAuthType:     "export-shared-secret",
	}
}

func (m *Middleware) matchExportSharedSecret(r *http.Request) bool {
	if m.exportSharedSecret == "" || !isExportReadPath(r) {
		return false
	}
	candidate := strings.TrimSpace(r.Header.Get(HeaderXExportToken))
	if candidate == "" {
		if bearer, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer "); ok {
			candidate = strings.TrimSpace(bearer)
		}
	}
	if candidate == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(candidate), []byte(m.exportSharedSecret)) == 1
}

func extractForwardedBearer(r *http.Request) string {
	if bearer, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer "); ok {
		if trimmed := strings.TrimSpace(bearer); trimmed != "" {
			return trimmed
		}
	}
	if tok := strings.TrimSpace(r.Header.Get(HeaderXBearerToken)); tok != "" {
		if bearer, ok := strings.CutPrefix(tok, "Bearer "); ok {
			return strings.TrimSpace(bearer)
		}
		return tok
	}
	return ""
}

// isBypassPath returns true when the request path does not require authentication.
func isBypassPath(path string) bool {
	return path == "/" || path == "/ui" || strings.HasPrefix(path, "/ui/") ||
		path == "/healthz" || path == "/readyz" || path == "/metrics" ||
		path == "/reports" || path == "/rates" || path == "/debug/dashboard"
}

func (m *Middleware) validateToken(bearer string) (jwt.MapClaims, error) {
	if m.keys == nil {
		return nil, errors.New("JWKS key store not initialized")
	}
	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(bearer, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		kid, _ := token.Header["kid"].(string)
		if kid == "" {
			return nil, errors.New("token missing kid header")
		}

		key, ok := m.keys.Load(kid)
		if !ok {
			if err := m.refreshKeys(); err != nil {
				return nil, fmt.Errorf("JWKS refresh failed: %w", err)
			}
			key, ok = m.keys.Load(kid)
			if !ok {
				return nil, fmt.Errorf("unknown kid: %s", kid)
			}
		}

		return key, nil
	}, jwt.WithIssuer(m.issuerURL), jwt.WithExpirationRequired())

	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, errors.New("token is not valid")
	}

	return claims, nil
}

type oidcDiscovery struct {
	JWKSURI string `json:"jwks_uri"`
	Issuer  string `json:"issuer"`
}

func (m *Middleware) discoverJWKS() (string, error) {
	url := strings.TrimRight(m.issuerURL, "/") + "/.well-known/openid-configuration"
	resp, err := m.client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "", fmt.Errorf("OIDC discovery returned %d: %s", resp.StatusCode, body)
	}

	var disc oidcDiscovery
	if err := json.NewDecoder(resp.Body).Decode(&disc); err != nil {
		return "", err
	}
	if disc.JWKSURI == "" {
		return "", errors.New("OIDC discovery response missing jwks_uri")
	}

	return disc.JWKSURI, nil
}

type jwksResponse struct {
	Keys []jwkKey `json:"keys"`
}

type jwkKey struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	N   string `json:"n"`
	E   string `json:"e"`
	Alg string `json:"alg"`
	Use string `json:"use"`
}

func (m *Middleware) refreshKeys() error {
	resp, err := m.client.Get(m.jwksURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("JWKS fetch returned %d: %s", resp.StatusCode, body)
	}

	var jwks jwksResponse
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return err
	}

	for _, key := range jwks.Keys {
		if key.Kty != "RSA" || key.Kid == "" {
			continue
		}

		rsaKey, parseErr := parseRSAPublicKey(key.N, key.E)
		if parseErr != nil {
			m.logger.Warn("failed to parse JWKS key", "kid", key.Kid, "error", parseErr)
			continue
		}

		m.keys.Store(key.Kid, rsaKey)
		m.logger.Debug("loaded JWKS key", "kid", key.Kid)
	}

	return nil
}

func writeAuthError(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
