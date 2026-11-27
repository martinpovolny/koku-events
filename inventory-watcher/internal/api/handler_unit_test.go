package api

import (
	"encoding/json"
	"log/slog"
	"testing"
	"time"
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
