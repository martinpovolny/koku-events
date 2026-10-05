package osac

import (
	"encoding/json"
	"testing"
)

func TestCatalogItemTemplateAcceptsString(t *testing.T) {
	var item CatalogItem
	if err := json.Unmarshal([]byte(`{"id":"item-1","template":"template-1"}`), &item); err != nil {
		t.Fatal(err)
	}
	if item.Template != "template-1" {
		t.Fatalf("expected template-1, got %q", item.Template)
	}
}

func TestCatalogItemTemplateAcceptsReference(t *testing.T) {
	var item CatalogItem
	if err := json.Unmarshal([]byte(`{"id":"item-1","template":{"id":"template-1","name":"small","shared":true}}`), &item); err != nil {
		t.Fatal(err)
	}
	if item.Template != "template-1" {
		t.Fatalf("expected template-1, got %q", item.Template)
	}
}

func TestCatalogItemTemplateUsesReferenceNameWhenIDMissing(t *testing.T) {
	var item CatalogItem
	if err := json.Unmarshal([]byte(`{"id":"item-1","template":{"name":"small"}}`), &item); err != nil {
		t.Fatal(err)
	}
	if item.Template != "small" {
		t.Fatalf("expected small, got %q", item.Template)
	}
}
