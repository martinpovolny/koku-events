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

func TestValidateCloudEvent_OSACv1BackdatedHeartbeatAllowed(t *testing.T) {
	h := &APIHandler{logger: slog.Default()}
	now := time.Now().UTC()
	from := now.Add(-36 * time.Hour).Truncate(time.Second)
	to := from.Add(time.Hour)
	data := json.RawMessage(`{
		"schema_version": "v1",
		"tenant_id": "tenant-1",
		"external_billing_account_id": "acct-1",
		"resource_type": "volume",
		"resource_id": "vol-1",
		"meter": "capacity",
		"component": "volume",
		"heartbeat_time": "` + now.Format(time.RFC3339) + `",
		"usage": {
			"semantics": "interval",
			"from": "` + from.Format(time.RFC3339) + `",
			"to": "` + to.Format(time.RFC3339) + `",
			"quantity": "-120.000000",
			"unit": "gibibyte_second",
			"precision": "second"
		}
	}`)
	event := cloudEventInternal{
		SpecVersion: "1.0", ID: "hb-backdated-1", Source: "osac-metering", Type: eventTypeResourceHeartbeat,
		DataContentType: "application/json", Time: from, Data: data,
		OSACResourceID: "vol-1", OSACResourceType: "volume", OSACTenant: "tenant-1",
	}
	if err := h.validateCloudEvent(event); err != nil {
		t.Fatalf("backdated OSAC v1 interval correction heartbeat rejected: %v", err)
	}
}

func TestValidateCloudEvent_OSACv1FutureHeartbeatRejected(t *testing.T) {
	h := &APIHandler{logger: slog.Default()}
	now := time.Now().UTC()
	future := now.Add(15 * time.Minute).Truncate(time.Second)
	data := json.RawMessage(`{
		"schema_version": "v1",
		"tenant_id": "tenant-1",
		"resource_type": "compute_instance",
		"resource_id": "vm-1",
		"meter": "compute",
		"component": "compute",
		"heartbeat_time": "` + future.Format(time.RFC3339) + `",
		"usage": {
			"semantics": "interval",
			"from": "` + now.Add(-5*time.Minute).Format(time.RFC3339) + `",
			"to": "` + now.Format(time.RFC3339) + `",
			"quantity": "300.000000",
			"unit": "seconds",
			"precision": "second"
		}
	}`)
	event := cloudEventInternal{
		SpecVersion: "1.0", ID: "hb-future-1", Source: "osac-metering", Type: eventTypeResourceHeartbeat,
		DataContentType: "application/json", Time: now.Add(-5 * time.Minute), Data: data,
		OSACResourceID: "vm-1", OSACResourceType: "compute_instance", OSACTenant: "tenant-1",
	}
	if err := h.validateCloudEvent(event); err == nil {
		t.Fatal("expected future heartbeat_time to be rejected")
	}
}

func TestValidateCloudEvent_UnsupportedOSACVersionRejected(t *testing.T) {
	h := &APIHandler{logger: slog.Default()}
	event := cloudEventInternal{
		SpecVersion: "1.0", ID: "hb-v2-1", Source: "osac-metering", Type: "osac.resource.heartbeat.v2",
		DataContentType: "application/json", Time: time.Now().UTC(), Data: json.RawMessage(`{"tenant_id":"t1"}`),
	}
	err := h.validateCloudEvent(event)
	if err == nil {
		t.Fatal("expected osac.resource.heartbeat.v2 to be rejected")
	}
	valErr, ok := err.(*eventValidationError)
	if !ok || valErr.DeadLetterReason() != "unsupported_version" {
		t.Fatalf("expected unsupported_version validation error, got %T (%v)", err, err)
	}
}

func TestCloudEventDigest_MatchesAcrossRawKafkaAndLockedAdapterPaths(t *testing.T) {
	ts := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)

	// Raw OSAC Kafka heartbeat (no schema_version, includes OSAC-internal fields).
	rawKafka := cloudEventInternal{
		SpecVersion: "1.0", ID: "hb-cutover-1", Source: "osac-metering", Type: eventTypeResourceHeartbeat,
		DataContentType: "application/json", Time: ts,
		OSACResourceID: "vm-1", OSACResourceType: "compute_instance", OSACTenant: "tenant-1", OSACProject: "proj-1",
		Data: json.RawMessage(`{
			"tenant_id": "tenant-1",
			"external_billing_account_id": "acct-1",
			"resource_type": "compute_instance",
			"resource_id": "vm-1",
			"meter": "compute",
			"component": "compute",
			"heartbeat_time": "2026-10-01T12:05:00Z",
			"last_applied_offset": 42,
			"rules_revision": "rev-7",
			"billing_dimensions": {"project": "proj-1", "instance_type": "standard-4-8"},
			"usage": {"semantics": "interval", "from": "2026-10-01T12:00:00Z", "to": "2026-10-01T12:05:00Z", "quantity": "300.000000", "unit": "seconds", "precision": "second"}
		}`),
	}

	// Adapter-mapped locked HTTP CloudEventEnvelope (schema_version="v1", internal fields stripped).
	lockedHTTP := cloudEventInternal{
		SpecVersion: "1.0", ID: "hb-cutover-1", Source: "osac-metering", Type: eventTypeResourceHeartbeat,
		DataContentType: "application/json", Time: ts,
		OSACResourceID: "vm-1", OSACResourceType: "compute_instance", OSACTenant: "tenant-1", OSACProject: "proj-1",
		Data: json.RawMessage(`{
			"schema_version": "v1",
			"tenant_id": "tenant-1",
			"external_billing_account_id": "acct-1",
			"project_id": "proj-1",
			"resource_type": "compute_instance",
			"resource_id": "vm-1",
			"meter": "compute",
			"component": "compute",
			"heartbeat_time": "2026-10-01T12:05:00Z",
			"usage": {"semantics": "interval", "from": "2026-10-01T12:00:00Z", "to": "2026-10-01T12:05:00Z", "quantity": "300.000000", "unit": "seconds", "precision": "second"},
			"billing_dimensions": {"instance_type": "standard-4-8", "project": "proj-1"}
		}`),
	}

	// Genuine collision: same identity, changed quantity.
	collision := rawKafka
	collision.Data = json.RawMessage(`{
		"tenant_id": "tenant-1",
		"external_billing_account_id": "acct-1",
		"resource_type": "compute_instance",
		"resource_id": "vm-1",
		"meter": "compute",
		"component": "compute",
		"heartbeat_time": "2026-10-01T12:05:00Z",
		"billing_dimensions": {"project": "proj-1", "instance_type": "standard-4-8"},
		"usage": {"semantics": "interval", "from": "2026-10-01T12:00:00Z", "to": "2026-10-01T12:05:00Z", "quantity": "999.000000", "unit": "seconds", "precision": "second"}
	}`)

	dRaw, err := cloudEventDigest(rawKafka)
	if err != nil {
		t.Fatal(err)
	}
	dLocked, err := cloudEventDigest(lockedHTTP)
	if err != nil {
		t.Fatal(err)
	}
	dCollision, err := cloudEventDigest(collision)
	if err != nil {
		t.Fatal(err)
	}
	if dRaw != dLocked {
		t.Fatalf("expected identical receipt digest across raw Kafka and locked HTTP paths: %s != %s", dRaw, dLocked)
	}
	if dRaw == dCollision {
		t.Fatalf("expected different digest when usage quantity changed")
	}

	// Sub-second time differences must also produce distinct digests.
	subsec := rawKafka
	subsec.Time = ts.Add(123 * time.Millisecond)
	dSubsec, err := cloudEventDigest(subsec)
	if err != nil {
		t.Fatal(err)
	}
	if dRaw == dSubsec {
		t.Fatalf("expected different digest when sub-second time differs")
	}
}

func TestValidateCloudEvent_RejectsMalformedExplicitUsageOnInference(t *testing.T) {
	h := &APIHandler{logger: slog.Default()}
	event := cloudEventInternal{
		SpecVersion: "1.0", ID: "inf-malformed-1", Source: "osac-metering", Type: eventTypeInferenceUsage,
		DataContentType: "application/json", Time: time.Now().UTC(),
		OSACResourceID: "m-1", OSACResourceType: "maas_inference", OSACTenant: "tenant-1",
		// Numeric usage.quantity + nonzero prompt_tokens must NOT fall back to legacy path.
		Data: json.RawMessage(`{
			"tenant_id": "tenant-1",
			"resource_type": "maas_inference",
			"resource_id": "m-1",
			"usage": {"quantity": 1200, "unit": "tokens"},
			"billing_dimensions": {"model": "llama-3", "prompt_tokens": 100}
		}`),
	}
	if err := h.validateCloudEvent(event); err == nil {
		t.Fatal("expected malformed explicit usage object to fail validation instead of falling back to legacy path")
	}
}

func TestClassifyEvent_LegacyHeartbeatRetainsInternalTenant(t *testing.T) {
	// Legacy lifecycle heartbeat (has current_state, no usage) must not override
	// raw_events.tenant_id with external_billing_account_id since inventory upsert uses tenant_id.
	legacyHB := cloudEventInternal{
		SpecVersion: "1.0", ID: "legacy-hb-1", Source: "osac-metering", Type: eventTypeResourceHeartbeat,
		Time: time.Now().UTC(), OSACResourceID: "vm-1", OSACResourceType: "compute_instance", OSACTenant: "tenant-internal",
		Data: json.RawMessage(`{"resource_id":"vm-1","resource_type":"compute_instance","tenant_id":"tenant-internal","external_billing_account_id":"acct-ext","current_state":"RUNNING"}`),
	}
	_, _, tenant := classifyEvent(legacyHB)
	if tenant != "tenant-internal" {
		t.Fatalf("legacy heartbeat tenant = %q, want tenant-internal", tenant)
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
