package reconciler

import (
	"log/slog"
	"os"
	"testing"
	"time"
)

func TestReconciler_IsEntityEnabled(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	t.Run("default empty enables all entities", func(t *testing.T) {
		r := New(nil, nil, nil, time.Hour, logger)
		entities := []string{
			"projects", "tenants", "compute_instances", "clusters",
			"instance_types", "bare_metal_instances", "catalog_items",
		}
		for _, e := range entities {
			if !r.IsEntityEnabled(e) {
				t.Errorf("expected %q to be enabled by default", e)
			}
		}
	})

	t.Run("wildcard 'all' enables all entities", func(t *testing.T) {
		r := New(nil, nil, nil, time.Hour, logger)
		r.SetEntities(map[string]bool{"all": true})
		if !r.IsEntityEnabled("catalog_items") {
			t.Error("expected catalog_items to be enabled with 'all'")
		}
		if !r.IsEntityEnabled("compute_instances") {
			t.Error("expected compute_instances to be enabled with 'all'")
		}
	})

	t.Run("wildcard '*' enables all entities", func(t *testing.T) {
		r := New(nil, nil, nil, time.Hour, logger)
		r.SetEntities(map[string]bool{"*": true})
		if !r.IsEntityEnabled("clusters") {
			t.Error("expected clusters to be enabled with '*'")
		}
	})

	t.Run("selective catalog and support entities filter", func(t *testing.T) {
		r := New(nil, nil, nil, time.Hour, logger)
		r.SetEntities(map[string]bool{
			"catalog_items":  true,
			"instance_types": true,
			"tenants":        true,
			"projects":       true,
		})

		// Enabled entities
		enabled := []string{"catalog_items", "instance_types", "tenants", "projects"}
		for _, e := range enabled {
			if !r.IsEntityEnabled(e) {
				t.Errorf("expected %q to be enabled", e)
			}
		}

		// Case insensitivity
		if !r.IsEntityEnabled("CATALOG_ITEMS") {
			t.Error("expected CATALOG_ITEMS to be enabled case-insensitively")
		}
		if !r.IsEntityEnabled("Instance_Types") {
			t.Error("expected Instance_Types to be enabled case-insensitively")
		}

		// Disabled workload entities
		disabled := []string{"compute_instances", "clusters", "bare_metal_instances"}
		for _, e := range disabled {
			if r.IsEntityEnabled(e) {
				t.Errorf("expected %q to be disabled", e)
			}
		}
	})
}
