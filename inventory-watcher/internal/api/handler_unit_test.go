package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/osac-project/cost-event-consumer/internal/authn"
)

func TestValidateCloudEventAcceptsCanonicalOSACv1(t *testing.T) {
	h := &APIHandler{logger: slog.Default()}
	data, err := json.Marshal(meteringData{
		ResourceID: "vm-1", ResourceType: "compute_instance", TenantID: "tenant-1",
		CurrentState: "RUNNING", TransitionTime: time.Now().UTC().Format(time.RFC3339),
		SchemaVersion: "v1",
	})
	if err != nil {
		t.Fatal(err)
	}
	event := cloudEventInternal{
		SpecVersion: "1.0", ID: "event-1", Source: "osac-metering", Type: eventTypeResourceCreated,
		Time: time.Now().UTC(), Data: data,
		OSACResourceID: "vm-1", OSACResourceType: "compute_instance", OSACTenant: "tenant-1",
	}
	if err := h.validateCloudEvent(event); err != nil {
		t.Fatalf("canonical OSAC v1 event rejected: %v", err)
	}
}

func TestValidateCloudEventRejectsMismatchedOSACv1Identity(t *testing.T) {
	h := &APIHandler{logger: slog.Default()}
	data := json.RawMessage(`{"resource_id":"vm-2","resource_type":"compute_instance","tenant_id":"tenant-1","current_state":"RUNNING","schema_version":"v1"}`)
	event := cloudEventInternal{
		SpecVersion: "1.0", ID: "event-2", Source: "osac-metering", Type: eventTypeResourceCreated,
		Time: time.Now().UTC(), Data: data,
		OSACResourceID: "vm-1", OSACResourceType: "compute_instance", OSACTenant: "tenant-1",
	}
	if err := h.validateCloudEvent(event); err == nil {
		t.Fatal("mismatched OSAC v1 identity was accepted")
	}
}

func TestCloudEventDigestIsStable(t *testing.T) {
	event := cloudEventInternal{
		SpecVersion: "1.0", ID: "event-3", Source: "osac-metering", Type: eventTypeResourceCreated,
		Time: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), Data: json.RawMessage(`{"resource_id":"vm-1"}`),
	}
	first, err := cloudEventDigest(event)
	if err != nil {
		t.Fatal(err)
	}
	second, err := cloudEventDigest(event)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("digest is not stable: %s != %s", first, second)
	}
}

// TestEnforceTenant_Unauthenticated verifies enforceTenant allows unauthenticated requests in dev mode.
func TestEnforceTenant_Unauthenticated(t *testing.T) {
	ctx := context.Background()
	tenant := "any-tenant"
	if err := enforceTenant(ctx, &tenant); err != nil {
		t.Fatalf("expected nil error in unauthenticated context, got %v", err)
	}
	if tenant != "any-tenant" {
		t.Errorf("expected tenant to remain unchanged, got %s", tenant)
	}

	emptyTenant := ""
	if err := enforceTenant(ctx, &emptyTenant); err != nil {
		t.Fatalf("expected nil error for empty tenant in unauthenticated context, got %v", err)
	}
	if emptyTenant != "" {
		t.Errorf("expected empty tenant to remain empty, got %s", emptyTenant)
	}
}

// TestEnforceTenant_OrgAdmin verifies organization admins are scoped to their own OrgID.
func TestEnforceTenant_OrgAdmin(t *testing.T) {
	adminCtx := authn.WithIdentity(context.Background(), &authn.Identity{
		OrgID:      "admin-org",
		Username:   "org-admin",
		IsOrgAdmin: true,
	})

	// 1. Same tenant succeeds
	matchingTenant := "admin-org"
	if err := enforceTenant(adminCtx, &matchingTenant); err != nil {
		t.Fatalf("expected matching tenant to succeed for org admin, got %v", err)
	}
	if matchingTenant != "admin-org" {
		t.Errorf("expected admin-org, got %s", matchingTenant)
	}

	// 2. Mismatched tenant is forbidden (IsOrgAdmin is scoped to their own OrgID)
	otherTenant := "other-tenant"
	if err := enforceTenant(adminCtx, &otherTenant); err == nil {
		t.Fatal("expected error for mismatched tenant with org admin, got nil")
	}

	// 3. Empty tenant is populated with admin's OrgID
	emptyTenant := ""
	if err := enforceTenant(adminCtx, &emptyTenant); err != nil {
		t.Fatalf("expected empty tenant to be populated with admin OrgID, got %v", err)
	}
	if emptyTenant != "admin-org" {
		t.Errorf("expected emptyTenant to be populated as admin-org, got %s", emptyTenant)
	}
}

// TestEnforceTenant_PlatformAdmin verifies platform admins can query across all tenants.
func TestEnforceTenant_PlatformAdmin(t *testing.T) {
	platformCtx := authn.WithIdentity(context.Background(), &authn.Identity{
		OrgID:           "platform-org",
		Username:        "superadmin",
		IsPlatformAdmin: true,
	})

	// 1. Can query any tenant
	tenant := "any-customer-org"
	if err := enforceTenant(platformCtx, &tenant); err != nil {
		t.Fatalf("platform admin should be able to query any tenant, got err %v", err)
	}
	if tenant != "any-customer-org" {
		t.Errorf("expected any-customer-org, got %s", tenant)
	}

	// 2. Can leave tenant empty for cross-tenant / global query
	emptyTenant := ""
	if err := enforceTenant(platformCtx, &emptyTenant); err != nil {
		t.Fatalf("platform admin should be able to leave tenant empty, got err %v", err)
	}
	if emptyTenant != "" {
		t.Errorf("expected emptyTenant to stay empty for platform admin global queries, got %s", emptyTenant)
	}
}

// TestEnforceTenant_EmptyOrgID verifies non-platform identities without an OrgID are rejected.
func TestEnforceTenant_EmptyOrgID(t *testing.T) {
	ctx := authn.WithIdentity(context.Background(), &authn.Identity{
		OrgID:    "",
		Username: "no-org-user",
	})

	tenant := "some-tenant"
	if err := enforceTenant(ctx, &tenant); err == nil {
		t.Fatal("expected error when identity has empty OrgID, got nil")
	}
}

// TestEnforceTenant_RegularUser verifies regular users are restricted to their authenticated OrgID.
func TestEnforceTenant_RegularUser(t *testing.T) {
	userCtx := authn.WithIdentity(context.Background(), &authn.Identity{
		OrgID:      "tenant-100",
		Username:   "regular-user",
		IsOrgAdmin: false,
	})

	// 1. Same tenant succeeds
	matchingTenant := "tenant-100"
	if err := enforceTenant(userCtx, &matchingTenant); err != nil {
		t.Fatalf("expected matching tenant to succeed, got %v", err)
	}
	if matchingTenant != "tenant-100" {
		t.Errorf("expected tenant-100, got %s", matchingTenant)
	}

	// 2. Mismatched tenant is forbidden
	mismatchedTenant := "tenant-200"
	if err := enforceTenant(userCtx, &mismatchedTenant); err == nil {
		t.Fatal("expected error for mismatched tenant, got nil")
	}

	// 3. Empty tenant is automatically populated to the user's OrgID
	emptyTenant := ""
	if err := enforceTenant(userCtx, &emptyTenant); err != nil {
		t.Fatalf("expected empty tenant to be populated with user OrgID, got %v", err)
	}
	if emptyTenant != "tenant-100" {
		t.Errorf("expected tenant to be populated as tenant-100, got %s", emptyTenant)
	}
}

// TestProcessEvents_TenantIsolation verifies tenant isolation during event batch ingestion.
func TestProcessEvents_TenantIsolation(t *testing.T) {
	h := &APIHandler{logger: slog.Default()}

	dataTenant1, _ := json.Marshal(meteringData{
		ResourceID: "vm-1", ResourceType: "compute_instance", TenantID: "tenant-1",
		CurrentState: "RUNNING", TransitionTime: time.Now().UTC().Format(time.RFC3339),
		SchemaVersion: "v1",
	})
	eventTenant1 := cloudEventInternal{
		SpecVersion: "1.0", ID: "event-t1", Source: "osac-metering", Type: eventTypeResourceCreated,
		Time: time.Now().UTC(), Data: dataTenant1,
		OSACResourceID: "vm-1", OSACResourceType: "compute_instance", OSACTenant: "tenant-1",
	}

	// Case 1: Caller authenticated as tenant-2 tries to ingest event for tenant-1
	ctxTenant2 := authn.WithIdentity(context.Background(), &authn.Identity{
		OrgID:      "tenant-2",
		Username:   "user2",
		IsOrgAdmin: false,
	})
	err := h.processEvents(ctxTenant2, []cloudEventInternal{eventTenant1})
	if err == nil {
		t.Fatal("expected tenant mismatch error, got nil")
	}
	var valErr *eventValidationError
	if !errors.As(err, &valErr) {
		t.Fatalf("expected *eventValidationError, got %T: %v", err, err)
	}

	// Case 2: Caller authenticated as tenant-1 ingests event for tenant-1
	ctxTenant1 := authn.WithIdentity(context.Background(), &authn.Identity{
		OrgID:      "tenant-1",
		Username:   "user1",
		IsOrgAdmin: false,
	})
	err1 := h.processEvents(ctxTenant1, []cloudEventInternal{eventTenant1})
	if err1 != nil {
		t.Fatalf("expected nil error for matching tenant, got %v", err1)
	}

	// Case 3: Event with empty tenant is rejected for non-platform-admin
	dataEmptyTenant, _ := json.Marshal(meteringData{
		ResourceID: "vm-2", ResourceType: "compute_instance", TenantID: "",
		CurrentState: "RUNNING", TransitionTime: time.Now().UTC().Format(time.RFC3339),
		SchemaVersion: "v1",
	})
	eventEmptyTenant := cloudEventInternal{
		SpecVersion: "1.0", ID: "event-empty", Source: "osac-metering", Type: eventTypeResourceCreated,
		Time: time.Now().UTC(), Data: dataEmptyTenant,
		OSACResourceID: "vm-2", OSACResourceType: "compute_instance", OSACTenant: "",
	}
	errEmpty := h.processEvents(ctxTenant1, []cloudEventInternal{eventEmptyTenant})
	if errEmpty == nil {
		t.Fatal("expected error for empty event tenant, got nil")
	}
	if !errors.As(errEmpty, &valErr) {
		t.Fatalf("expected *eventValidationError for empty event tenant, got %T: %v", errEmpty, errEmpty)
	}

	// Case 4: Org admin of tenant-2 cannot ingest events for tenant-1
	ctxOrgAdmin := authn.WithIdentity(context.Background(), &authn.Identity{
		OrgID:      "tenant-2",
		Username:   "admin2",
		IsOrgAdmin: true,
	})
	errOrgAdmin := h.processEvents(ctxOrgAdmin, []cloudEventInternal{eventTenant1})
	if errOrgAdmin == nil {
		t.Fatal("expected tenant mismatch error for org admin of different tenant, got nil")
	}
	if !errors.As(errOrgAdmin, &valErr) {
		t.Fatalf("expected *eventValidationError, got %T: %v", errOrgAdmin, errOrgAdmin)
	}

	// Case 5: Platform admin can ingest events for tenant-1
	ctxPlatformAdmin := authn.WithIdentity(context.Background(), &authn.Identity{
		OrgID:           "platform-org",
		Username:        "superadmin",
		IsPlatformAdmin: true,
	})
	errPlatform := h.processEvents(ctxPlatformAdmin, []cloudEventInternal{eventTenant1})
	if errPlatform != nil {
		t.Fatalf("expected nil error for platform admin, got %v", errPlatform)
	}
}
