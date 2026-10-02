package authn

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
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
