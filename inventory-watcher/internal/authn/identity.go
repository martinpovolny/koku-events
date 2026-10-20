package authn

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// Identity represents the authenticated principal (user, service account, or system).
type Identity struct {
	OrgID         string `json:"org_id"`
	AccountNumber string `json:"account_number,omitempty"`
	Username      string `json:"username,omitempty"`
	Email         string `json:"email,omitempty"`
	IsOrgAdmin    bool   `json:"is_org_admin"`
	RawAuthType   string `json:"raw_auth_type,omitempty"` // "x-rh-identity" or "bearer-jwt"
}

const (
	identityKey contextKey = "auth_identity"

	// HeaderXRhIdentity is the Red Hat Insights identity header name.
	HeaderXRhIdentity = "X-Rh-Identity"
)

// WithIdentity stores the authenticated Identity into the context.
func WithIdentity(ctx context.Context, id *Identity) context.Context {
	return context.WithValue(ctx, identityKey, id)
}

// IdentityFromContext extracts the authenticated Identity from the context.
func IdentityFromContext(ctx context.Context) (*Identity, bool) {
	id, ok := ctx.Value(identityKey).(*Identity)
	return id, ok && id != nil
}

// xRhIdentityPayload defines the wire format of the X-Rh-Identity JSON header.
type xRhIdentityPayload struct {
	OrgID    string `json:"org_id"`
	Identity struct {
		OrgID          string `json:"org_id"`
		AccountNumber  string `json:"account_number"`
		Type           string `json:"type"`
		User           struct {
			Username   string `json:"username"`
			Email      string `json:"email"`
			IsOrgAdmin bool   `json:"is_org_admin"`
		} `json:"user"`
		ServiceAccount struct {
			Username string `json:"username"`
			ClientID string `json:"client_id"`
		} `json:"service_account"`
		System struct {
			CommonName string `json:"cn"`
		} `json:"system"`
	} `json:"identity"`
	Entitlements map[string]struct {
		IsEntitled bool `json:"is_entitled"`
	} `json:"entitlements"`
}

// ParseXRhIdentity parses and validates a base64-encoded X-Rh-Identity header string.
func ParseXRhIdentity(encoded string) (*Identity, error) {
	encoded = strings.TrimSpace(encoded)
	if encoded == "" {
		return nil, errors.New("empty X-Rh-Identity header")
	}

	var raw []byte
	var err error
	// Support standard and URL encodings, with or without padding
	for _, enc := range []*base64.Encoding{
		base64.StdEncoding,
		base64.URLEncoding,
		base64.RawStdEncoding,
		base64.RawURLEncoding,
	} {
		raw, err = enc.DecodeString(encoded)
		if err == nil {
			break
		}
	}
	if err != nil {
		return nil, fmt.Errorf("decoding base64 X-Rh-Identity: %w", err)
	}

	var payload xRhIdentityPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("unmarshaling X-Rh-Identity JSON: %w", err)
	}

	orgID := strings.TrimSpace(payload.OrgID)
	if orgID == "" {
		orgID = strings.TrimSpace(payload.Identity.OrgID)
	}
	if orgID == "" {
		return nil, errors.New("missing org_id in X-Rh-Identity")
	}

	acct := strings.TrimSpace(payload.Identity.AccountNumber)
	user := strings.TrimSpace(payload.Identity.User.Username)
	if user == "" {
		user = strings.TrimSpace(payload.Identity.ServiceAccount.Username)
		if user == "" {
			user = strings.TrimSpace(payload.Identity.ServiceAccount.ClientID)
			if user == "" {
				user = strings.TrimSpace(payload.Identity.System.CommonName)
			}
		}
	}

	email := strings.TrimSpace(payload.Identity.User.Email)
	isAdmin := payload.Identity.User.IsOrgAdmin

	return &Identity{
		OrgID:         orgID,
		AccountNumber: acct,
		Username:      user,
		Email:         email,
		IsOrgAdmin:    isAdmin,
		RawAuthType:   "x-rh-identity",
	}, nil
}

// BuildXRhIdentity serializes an Identity into a standard base64-encoded X-Rh-Identity string.
func BuildXRhIdentity(id Identity) (string, error) {
	if strings.TrimSpace(id.OrgID) == "" {
		return "", errors.New("org_id is required")
	}

	username := id.Username
	if username == "" {
		username = "user"
	}
	email := id.Email
	if email == "" {
		email = username + "@example.com"
	}

	var payload xRhIdentityPayload
	payload.OrgID = id.OrgID
	payload.Identity.OrgID = id.OrgID
	payload.Identity.AccountNumber = id.AccountNumber
	payload.Identity.Type = "User"
	payload.Identity.User.Username = username
	payload.Identity.User.Email = email
	payload.Identity.User.IsOrgAdmin = id.IsOrgAdmin
	payload.Entitlements = map[string]struct {
		IsEntitled bool `json:"is_entitled"`
	}{
		"cost_management": {IsEntitled: true},
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshaling X-Rh-Identity: %w", err)
	}
	return base64.StdEncoding.EncodeToString(raw), nil
}

// IdentityFromJWTClaims constructs an Identity from parsed JWT map claims.
func IdentityFromJWTClaims(claims jwt.MapClaims) *Identity {
	id := &Identity{
		RawAuthType: "bearer-jwt",
	}

	if orgID, ok := claims["org_id"].(string); ok && orgID != "" {
		id.OrgID = orgID
	} else if orgNum, ok := claims["org_id"].(float64); ok {
		id.OrgID = fmt.Sprintf("%.0f", orgNum)
	}

	if acct, ok := claims["account_number"].(string); ok {
		id.AccountNumber = acct
	} else if acctNum, ok := claims["account_number"].(float64); ok {
		id.AccountNumber = fmt.Sprintf("%.0f", acctNum)
	}

	if user, ok := claims["preferred_username"].(string); ok && user != "" {
		id.Username = user
	} else if sub, ok := claims["sub"].(string); ok {
		id.Username = sub
	}

	if email, ok := claims["email"].(string); ok {
		id.Email = email
	}

	if admin, ok := claims["is_org_admin"].(bool); ok {
		id.IsOrgAdmin = admin
	} else if realmAccess, ok := claims["realm_access"].(map[string]interface{}); ok {
		if roles, ok := realmAccess["roles"].([]interface{}); ok {
			for _, r := range roles {
				if rStr, ok := r.(string); ok && (rStr == "org-admin" || rStr == "admin") {
					id.IsOrgAdmin = true
					break
				}
			}
		}
	}

	return id
}
