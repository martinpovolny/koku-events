package authn

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

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

func TestIdentityFromJWTClaims(t *testing.T) {
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
		t.Errorf("expected IsOrgAdmin true from realm role, got false")
	}
}

func TestAuthMiddleware_XRhIdentity(t *testing.T) {
	m := &Middleware{
		issuerURL: "https://auth.example.com",
		logger:    slog.Default(),
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

	t.Run("valid X-Rh-Identity passes", func(t *testing.T) {
		capturedID = nil
		encoded, err := BuildXRhIdentity(Identity{
			OrgID:      "tenant-42",
			Username:   "carol",
			IsOrgAdmin: false,
		})
		if err != nil {
			t.Fatal(err)
		}

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

	t.Run("invalid X-Rh-Identity returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/reports/costs", nil)
		req.Header.Set("X-Rh-Identity", "invalid-base64")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
		if w.Result().StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected status 401, got %d", w.Result().StatusCode)
		}
	})
}

func TestAuthMiddleware_DisabledWithIdentity(t *testing.T) {
	m := &Middleware{
		issuerURL: "", // disabled
		logger:    slog.Default(),
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

	encoded, _ := BuildXRhIdentity(Identity{
		OrgID:      "dev-tenant",
		Username:   "dev-user",
		IsOrgAdmin: true,
	})

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
}
