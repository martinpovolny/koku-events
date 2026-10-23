package authn

import (
	"crypto/rand"
	"crypto/rsa"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestAuthMiddleware_BypassPaths(t *testing.T) {
	m := &Middleware{
		issuerURL: "https://auth.example.com",
		logger:    slog.Default(),
	}

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	handler := m.Wrap(next)

	bypassPaths := []string{
		"/",
		"/ui",
		"/ui/rates",
		"/ui/reports",
		"/ui/dashboard",
		"/reports",
		"/rates",
		"/debug/dashboard",
		"/healthz",
		"/readyz",
		"/metrics",
	}

	for _, path := range bypassPaths {
		t.Run("bypass "+path, func(t *testing.T) {
			nextCalled = false
			req := httptest.NewRequest(http.MethodGet, path, nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if !nextCalled {
				t.Fatalf("expected next handler to be called for bypass path %s", path)
			}
			if w.Result().StatusCode != http.StatusOK {
				t.Errorf("expected status 200, got %d", w.Result().StatusCode)
			}
		})
	}

	protectedPaths := []string{
		"/api/v1/events",
		"/api/v1/rates",
		"/api/v1/catalog",
		"/api/v1/reports/costs",
	}

	for _, path := range protectedPaths {
		t.Run("protect "+path, func(t *testing.T) {
			nextCalled = false
			req := httptest.NewRequest(http.MethodGet, path, nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if nextCalled {
				t.Fatalf("did not expect next handler to be called for protected path %s without token", path)
			}
			if w.Result().StatusCode != http.StatusUnauthorized {
				t.Errorf("expected status 401, got %d", w.Result().StatusCode)
			}
		})
	}
}

// TestParseXRhIdentity_Valid verifies valid base64-encoded X-Rh-Identity headers parse accurately.
func TestParseXRhIdentity_Valid(t *testing.T) {
	headerVal, err := BuildXRhIdentity(Identity{
		OrgID:         "org-12345",
		AccountNumber: "acct-67890",
		Username:      "alice",
		Email:         "alice@example.com",
		IsOrgAdmin:    true,
	})
	if err != nil {
		t.Fatalf("BuildXRhIdentity failed: %v", err)
	}

	id, err := ParseXRhIdentity(headerVal)
	if err != nil {
		t.Fatalf("ParseXRhIdentity failed: %v", err)
	}

	if id.OrgID != "org-12345" {
		t.Errorf("expected OrgID org-12345, got %s", id.OrgID)
	}
	if id.AccountNumber != "acct-67890" {
		t.Errorf("expected AccountNumber acct-67890, got %s", id.AccountNumber)
	}
	if id.Username != "alice" {
		t.Errorf("expected Username alice, got %s", id.Username)
	}
	if id.Email != "alice@example.com" {
		t.Errorf("expected Email alice@example.com, got %s", id.Email)
	}
	if !id.IsOrgAdmin {
		t.Errorf("expected IsOrgAdmin true, got false")
	}
	if id.RawAuthType != "x-rh-identity" {
		t.Errorf("expected RawAuthType x-rh-identity, got %s", id.RawAuthType)
	}
}

// TestParseXRhIdentity_Invalid verifies malformed or incomplete X-Rh-Identity headers return an error.
func TestParseXRhIdentity_Invalid(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{"empty", ""},
		{"whitespace", "   "},
		{"not base64", "this-is-not-base64-%%"},
		{"invalid json", "bm90LWpzb24="}, // "not-json" in base64
		{"missing org_id", "eyJpZGVudGl0eSI6eyJ1c2VyIjp7InVzZXJuYW1lIjoidGVzdCJ9fX0="}, // {"identity":{"user":{"username":"test"}}}
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseXRhIdentity(tc.raw)
			if err == nil {
				t.Fatalf("expected error for case %s, got nil", tc.name)
			}
		})
	}
}

// TestIdentityFromJWTClaims verifies JWT claim extraction and role mapping.
func TestIdentityFromJWTClaims(t *testing.T) {
	t.Run("org-admin role sets IsOrgAdmin", func(t *testing.T) {
		claims := jwt.MapClaims{
			"org_id":             "org-999",
			"account_number":     "acct-888",
			"preferred_username": "bob",
			"email":              "bob@example.com",
			"realm_access": map[string]interface{}{
				"roles": []interface{}{"default-roles-kubernetes", "offline_access", "org-admin"},
			},
		}

		id := IdentityFromJWTClaims(claims)
		if id.OrgID != "org-999" {
			t.Errorf("expected OrgID org-999, got %s", id.OrgID)
		}
		if id.AccountNumber != "acct-888" {
			t.Errorf("expected AccountNumber acct-888, got %s", id.AccountNumber)
		}
		if id.Username != "bob" {
			t.Errorf("expected Username bob, got %s", id.Username)
		}
		if id.Email != "bob@example.com" {
			t.Errorf("expected Email bob@example.com, got %s", id.Email)
		}
		if !id.IsOrgAdmin {
			t.Errorf("expected IsOrgAdmin true from org-admin role, got false")
		}
		if id.IsPlatformAdmin {
			t.Errorf("expected IsPlatformAdmin false, got true")
		}
	})

	t.Run("generic admin role does NOT grant IsOrgAdmin or IsPlatformAdmin", func(t *testing.T) {
		claims := jwt.MapClaims{
			"org_id": "org-555",
			"realm_access": map[string]interface{}{
				"roles": []interface{}{"admin"},
			},
		}
		id := IdentityFromJWTClaims(claims)
		if id.IsOrgAdmin {
			t.Errorf("generic admin role should not grant IsOrgAdmin")
		}
		if id.IsPlatformAdmin {
			t.Errorf("generic admin role should not grant IsPlatformAdmin")
		}
	})

	t.Run("cost-platform-admin role grants IsPlatformAdmin", func(t *testing.T) {
		claims := jwt.MapClaims{
			"realm_access": map[string]interface{}{
				"roles": []interface{}{"cost-platform-admin"},
			},
		}
		id := IdentityFromJWTClaims(claims)
		if !id.IsPlatformAdmin {
			t.Errorf("expected IsPlatformAdmin true from cost-platform-admin role")
		}
	})

	t.Run("platform-admin role grants IsPlatformAdmin", func(t *testing.T) {
		claims := jwt.MapClaims{
			"realm_access": map[string]interface{}{
				"roles": []interface{}{"platform-admin"},
			},
		}
		id := IdentityFromJWTClaims(claims)
		if !id.IsPlatformAdmin {
			t.Errorf("expected IsPlatformAdmin true from platform-admin role")
		}
	})
}

// TestAuthMiddleware_XRhIdentity verifies X-Rh-Identity trust gating when auth is enabled.
func TestAuthMiddleware_XRhIdentity(t *testing.T) {
	encoded, err := BuildXRhIdentity(Identity{
		OrgID:      "tenant-42",
		Username:   "carol",
		IsOrgAdmin: false,
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("trusted gateway: valid X-Rh-Identity passes", func(t *testing.T) {
		var capturedID *Identity
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, ok := IdentityFromContext(r.Context())
			if ok {
				capturedID = id
			}
			w.WriteHeader(http.StatusOK)
		})

		m := &Middleware{
			issuerURL:           "https://auth.example.com",
			trustIdentityHeader: true,
			logger:              slog.Default(),
		}
		handler := m.Wrap(next)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/reports/costs", nil)
		req.Header.Set("X-Rh-Identity", encoded)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
		if w.Result().StatusCode != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Result().StatusCode)
		}
		if capturedID == nil {
			t.Fatal("expected Identity in request context")
		}
		if capturedID.OrgID != "tenant-42" || capturedID.Username != "carol" || capturedID.IsOrgAdmin {
			t.Errorf("unexpected identity fields: %+v", capturedID)
		}
	})

	t.Run("trusted gateway: invalid X-Rh-Identity returns 401", func(t *testing.T) {
		m := &Middleware{
			issuerURL:           "https://auth.example.com",
			trustIdentityHeader: true,
			logger:              slog.Default(),
		}
		handler := m.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

		req := httptest.NewRequest(http.MethodGet, "/api/v1/reports/costs", nil)
		req.Header.Set("X-Rh-Identity", "invalid-base64")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
		if w.Result().StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected status 401, got %d", w.Result().StatusCode)
		}
	})

	t.Run("untrusted gateway: X-Rh-Identity is ignored and falls through to bearer auth", func(t *testing.T) {
		m := &Middleware{
			issuerURL:           "https://auth.example.com",
			trustIdentityHeader: false, // default
			logger:              slog.Default(),
		}
		handler := m.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

		req := httptest.NewRequest(http.MethodGet, "/api/v1/reports/costs", nil)
		req.Header.Set("X-Rh-Identity", encoded)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
		if w.Result().StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected status 401, got %d", w.Result().StatusCode)
		}
	})
}

// TestAuthMiddleware_DisabledWithIdentity verifies X-Rh-Identity trust gating when auth is disabled.
func TestAuthMiddleware_DisabledWithIdentity(t *testing.T) {
	encoded, _ := BuildXRhIdentity(Identity{
		OrgID:      "dev-tenant",
		Username:   "dev-user",
		IsOrgAdmin: true,
	})

	t.Run("trust enabled attaches identity", func(t *testing.T) {
		m := &Middleware{
			issuerURL:           "", // disabled
			trustIdentityHeader: true,
			logger:              slog.Default(),
		}

		var capturedID *Identity
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, ok := IdentityFromContext(r.Context())
			if ok {
				capturedID = id
			}
			w.WriteHeader(http.StatusOK)
		})

		handler := m.Wrap(next)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/reports/costs", nil)
		req.Header.Set("X-Rh-Identity", encoded)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
		if w.Result().StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Result().StatusCode)
		}
		if capturedID == nil || capturedID.OrgID != "dev-tenant" || !capturedID.IsOrgAdmin {
			t.Fatalf("expected dev identity captured, got %+v", capturedID)
		}
	})

	t.Run("trust disabled ignores X-Rh-Identity", func(t *testing.T) {
		m := &Middleware{
			issuerURL:           "", // disabled
			trustIdentityHeader: false,
			logger:              slog.Default(),
		}

		var capturedID *Identity
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, ok := IdentityFromContext(r.Context())
			if ok {
				capturedID = id
			}
			w.WriteHeader(http.StatusOK)
		})

		handler := m.Wrap(next)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/reports/costs", nil)
		req.Header.Set("X-Rh-Identity", encoded)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
		if w.Result().StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Result().StatusCode)
		}
		if capturedID != nil {
			t.Fatalf("expected nil identity when trust is disabled, got %+v", capturedID)
		}
	})
}

// TestAuthMiddleware_BearerTokenValidation verifies direct Bearer JWT authentication and org_id enforcement.
func TestAuthMiddleware_BearerTokenValidation(t *testing.T) {
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	const issuer = "https://auth.example.com"
	keysMap := &sync.Map{}
	keysMap.Store("test-kid", &rsaKey.PublicKey)

	m := &Middleware{
		issuerURL:           issuer,
		trustIdentityHeader: false,
		keys:                keysMap,
		logger:              slog.Default(),
	}

	createToken := func(claims jwt.MapClaims) string {
		token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
		token.Header["kid"] = "test-kid"
		signed, err := token.SignedString(rsaKey)
		if err != nil {
			t.Fatalf("signing token: %v", err)
		}
		return signed
	}

	t.Run("valid token with org_id passes", func(t *testing.T) {
		var capturedID *Identity
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if id, ok := IdentityFromContext(r.Context()); ok {
				capturedID = id
			}
			w.WriteHeader(http.StatusOK)
		})

		handler := m.Wrap(next)
		tok := createToken(jwt.MapClaims{
			"iss":    issuer,
			"exp":    time.Now().Add(time.Hour).Unix(),
			"org_id": "org-token-123",
			"sub":    "user-sub",
		})

		req := httptest.NewRequest(http.MethodGet, "/api/v1/reports/costs", nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
		if w.Result().StatusCode != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Result().StatusCode)
		}
		if capturedID == nil || capturedID.OrgID != "org-token-123" {
			t.Fatalf("expected identity with OrgID org-token-123, got %+v", capturedID)
		}
	})

	t.Run("valid token without org_id is rejected with 401", func(t *testing.T) {
		handler := m.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		tok := createToken(jwt.MapClaims{
			"iss": issuer,
			"exp": time.Now().Add(time.Hour).Unix(),
			"sub": "user-without-org",
		})

		req := httptest.NewRequest(http.MethodGet, "/api/v1/reports/costs", nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
		if w.Result().StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected status 401 for token missing org_id, got %d", w.Result().StatusCode)
		}
	})

	t.Run("valid token without org_id but with platform-admin passes", func(t *testing.T) {
		var capturedID *Identity
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if id, ok := IdentityFromContext(r.Context()); ok {
				capturedID = id
			}
			w.WriteHeader(http.StatusOK)
		})

		handler := m.Wrap(next)
		tok := createToken(jwt.MapClaims{
			"iss": issuer,
			"exp": time.Now().Add(time.Hour).Unix(),
			"sub": "platform-admin-user",
			"realm_access": map[string]interface{}{
				"roles": []interface{}{"cost-platform-admin"},
			},
		})

		req := httptest.NewRequest(http.MethodGet, "/api/v1/reports/costs", nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
		if w.Result().StatusCode != http.StatusOK {
			t.Fatalf("expected status 200 for platform admin without org_id, got %d", w.Result().StatusCode)
		}
		if capturedID == nil || !capturedID.IsPlatformAdmin {
			t.Fatalf("expected IsPlatformAdmin true, got %+v", capturedID)
		}
	})
}
