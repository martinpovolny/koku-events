package osacmetering

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestParseRawEvent_RawOSACHeartbeatStripsInternalFields(t *testing.T) {
	// Matches upstream OSAC osac-metering schema.HeartbeatData delivered over Kafka
	// (no data.schema_version, includes OSAC-internal last_applied_offset & rules_revision).
	raw := []byte(`{
		"specversion": "1.0",
		"id": "sha256-hb-001",
		"source": "osac-metering",
		"type": "osac.resource.heartbeat.v1",
		"datacontenttype": "application/json",
		"time": "2026-10-01T12:00:00Z",
		"osacresourceid": "vm-123",
		"osacresourcetype": "compute_instance",
		"osactenant": "tenant-acme",
		"osacproject": "proj-alpha",
		"data": {
			"tenant_id": "tenant-acme",
			"external_billing_account_id": "acct-billing-99",
			"resource_type": "compute_instance",
			"resource_id": "vm-123",
			"meter": "compute",
			"component": "compute",
			"heartbeat_time": "2026-10-01T12:05:00Z",
			"last_applied_offset": 42,
			"rules_revision": "rev-7",
			"billing_dimensions": {
				"deployment": "cluster-a",
				"tenant": "tenant-acme",
				"project": "proj-alpha",
				"catalog": "cat-rhel-9",
				"instance_type": "standard-4-8"
			},
			"usage": {
				"semantics": "interval",
				"from": "2026-10-01T12:00:00Z",
				"to": "2026-10-01T12:05:00Z",
				"quantity": "300.000000",
				"unit": "seconds",
				"precision": "second"
			}
		}
	}`)

	got, err := ParseRawEvent(raw)
	if err != nil {
		t.Fatalf("ParseRawEvent failed: %v", err)
	}

	if got.Envelope.Data.SchemaVersion != ContractVersion {
		t.Errorf("schema_version = %q, want %q", got.Envelope.Data.SchemaVersion, ContractVersion)
	}
	if got.Envelope.Data.EffectiveTenantID() != "acct-billing-99" {
		t.Errorf("EffectiveTenantID = %q, want acct-billing-99", got.Envelope.Data.EffectiveTenantID())
	}
	if got.Envelope.Data.EffectiveProjectID() != "proj-alpha" {
		t.Errorf("EffectiveProjectID = %q, want proj-alpha", got.Envelope.Data.EffectiveProjectID())
	}
	if got.Quantity.StringFixed(6) != "300.000000" {
		t.Errorf("Quantity = %s, want 300.000000", got.Quantity.StringFixed(6))
	}

	// Verify locked envelope JSON excludes OSAC-internal fields.
	lockedBytes, err := json.Marshal(got.Envelope)
	if err != nil {
		t.Fatalf("marshal locked envelope: %v", err)
	}
	lockedJSON := string(lockedBytes)
	if strings.Contains(lockedJSON, "last_applied_offset") || strings.Contains(lockedJSON, "rules_revision") {
		t.Errorf("locked envelope leaked OSAC internal fields: %s", lockedJSON)
	}

	entry := got.ToMeteringEntry()
	if entry.TenantID != "acct-billing-99" {
		t.Errorf("entry.TenantID = %q, want acct-billing-99", entry.TenantID)
	}
	if entry.ProjectID != "proj-alpha" {
		t.Errorf("entry.ProjectID = %q, want proj-alpha", entry.ProjectID)
	}
	if entry.InstanceType != "standard-4-8" {
		t.Errorf("entry.InstanceType = %q, want standard-4-8", entry.InstanceType)
	}
	if entry.CatalogItem != "cat-rhel-9" {
		t.Errorf("entry.CatalogItem = %q, want cat-rhel-9", entry.CatalogItem)
	}
	if entry.ResourceType != "compute_instance" || entry.MeterName != "compute" || entry.Unit != "seconds" {
		t.Errorf("unexpected entry metadata: %+v", entry)
	}
	if entry.Value != 300.0 {
		t.Errorf("entry.Value = %v, want 300.0", entry.Value)
	}
	wantFrom := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	wantTo := time.Date(2026, 10, 1, 12, 5, 0, 0, time.UTC)
	if !entry.PeriodStart.Equal(wantFrom) || !entry.PeriodEnd.Equal(wantTo) {
		t.Errorf("period = [%v, %v], want [%v, %v]", entry.PeriodStart, entry.PeriodEnd, wantFrom, wantTo)
	}
}

func TestParseRawEvent_LockedAdapterEnvelopeAndNegativeCorrection(t *testing.T) {
	// Matches the locked wire payload sent by osac-project/osac#2661 over HTTP,
	// including a negative correction quantity for a backdated interval.
	raw := []byte(`{
		"specversion": "1.0",
		"id": "sha256-corr-001",
		"source": "osac-metering",
		"type": "osac.resource.heartbeat.v1",
		"datacontenttype": "application/json",
		"time": "2026-09-28T00:00:00Z",
		"osacresourceid": "vol-456",
		"osacresourcetype": "volume",
		"osactenant": "tenant-beta",
		"data": {
			"schema_version": "v1",
			"tenant_id": "tenant-beta",
			"resource_type": "volume",
			"resource_id": "vol-456",
			"meter": "capacity",
			"component": "volume",
			"heartbeat_time": "2026-10-01T12:05:00Z",
			"usage": {
				"semantics": "interval",
				"from": "2026-09-28T00:00:00Z",
				"to": "2026-09-28T01:00:00Z",
				"quantity": "-120.500000",
				"unit": "gibibyte_second",
				"precision": "second"
			},
			"billing_dimensions": {
				"project": "proj-storage",
				"storage_class": "gp3"
			}
		}
	}`)

	got, err := ParseRawEvent(raw)
	if err != nil {
		t.Fatalf("ParseRawEvent failed: %v", err)
	}

	if got.Envelope.Data.EffectiveTenantID() != "tenant-beta" {
		t.Errorf("EffectiveTenantID = %q, want tenant-beta", got.Envelope.Data.EffectiveTenantID())
	}
	if !got.Quantity.IsNegative() || got.Quantity.StringFixed(6) != "-120.500000" {
		t.Errorf("Quantity = %s, want -120.500000", got.Quantity.StringFixed(6))
	}
	entry := got.ToMeteringEntry()
	if entry.Value != -120.5 {
		t.Errorf("entry.Value = %v, want -120.5", entry.Value)
	}
	if entry.ProjectID != "proj-storage" {
		t.Errorf("entry.ProjectID = %q, want proj-storage", entry.ProjectID)
	}
}

func TestParseRawEvent_AllSixOSACResourceTypes(t *testing.T) {
	cases := []struct {
		resourceType string
		meter        string
		component    string
		unit         string
	}{
		{ResourceTypeComputeInstance, "compute", "compute", "seconds"},
		{ResourceTypeClusterOrder, "control_plane", "control_plane", "seconds"},
		{ResourceTypeBareMetalInstance, "compute", "bare_metal", "seconds"},
		{ResourceTypeExternalIP, "allocation", "external_ip", "seconds"},
		{ResourceTypeNATGateway, "consumption", "egress", "bytes"},
		{ResourceTypeVolume, "capacity", "volume", "gibibyte_second"},
	}

	for _, tc := range cases {
		t.Run(tc.resourceType, func(t *testing.T) {
			env := CloudEventEnvelope{
				SpecVersion:     CloudEventVersionV1,
				ID:              "evt-" + tc.resourceType,
				Source:          "osac-metering",
				Type:            EventTypeHeartbeat,
				DataContentType: ApplicationJSON,
				Time:            "2026-10-01T12:00:00Z",
				ResourceID:      "res-1",
				ResourceType:    tc.resourceType,
				Tenant:          "tenant-1",
				Data: UsageData{
					SchemaVersion: ContractVersion,
					TenantID:      "tenant-1",
					ResourceType:  tc.resourceType,
					ResourceID:    "res-1",
					Meter:         tc.meter,
					Component:     tc.component,
					HeartbeatTime: "2026-10-01T12:05:00Z",
					Usage: UsageInterval{
						Semantics: UsageSemanticsInterval,
						From:      "2026-10-01T12:00:00Z",
						To:        "2026-10-01T12:05:00Z",
						Quantity:  "60.000000",
						Unit:      tc.unit,
						Precision: "second",
					},
				},
			}
			raw, err := json.Marshal(env)
			if err != nil {
				t.Fatal(err)
			}
			got, err := ParseRawEvent(raw)
			if err != nil {
				t.Fatalf("resource_type %s rejected: %v", tc.resourceType, err)
			}
			entry := got.ToMeteringEntry()
			if entry.ResourceType != tc.resourceType || entry.MeterName != tc.meter || entry.Unit != tc.unit {
				t.Errorf("unexpected entry: %+v", entry)
			}
		})
	}
}

func TestParseRawEvent_InferenceUsage(t *testing.T) {
	raw := []byte(`{
		"specversion": "1.0",
		"id": "inf-001",
		"source": "osac-metering",
		"type": "osac.inference.usage.v1",
		"datacontenttype": "application/json",
		"time": "2026-10-01T12:00:00Z",
		"osacresourceid": "llama-3-70b",
		"osacresourcetype": "maas_inference",
		"osactenant": "tenant-ai",
		"data": {
			"tenant_id": "tenant-ai",
			"external_billing_account_id": "acct-ai",
			"resource_type": "maas_inference",
			"resource_id": "llama-3-70b",
			"usage": {
				"quantity": "1500",
				"unit": "tokens"
			},
			"billing_dimensions": {
				"model": "llama-3-70b",
				"project": "proj-llm"
			}
		}
	}`)

	got, err := ParseRawEvent(raw)
	if err != nil {
		t.Fatalf("ParseRawEvent inference failed: %v", err)
	}
	if got.Envelope.Data.Meter != "inference" || got.Envelope.Data.Usage.Semantics != UsageSemanticsDelta {
		t.Errorf("unexpected defaults: meter=%q semantics=%q", got.Envelope.Data.Meter, got.Envelope.Data.Usage.Semantics)
	}
	entry := got.ToMeteringEntry()
	if entry.TenantID != "acct-ai" || entry.ProjectID != "proj-llm" || entry.MeterName != "inference" || entry.Value != 1500 {
		t.Errorf("unexpected inference entry: %+v", entry)
	}
}

func TestParseRawEvent_ValidationAndVersionErrors(t *testing.T) {
	tests := []struct {
		name           string
		raw            string
		wantUnsupported bool
	}{
		{
			name: "future heartbeat event type v2",
			raw: `{
				"specversion":"1.0","id":"1","source":"osac-metering","type":"osac.resource.heartbeat.v2",
				"time":"2026-10-01T12:00:00Z","data":{"tenant_id":"t1"}
			}`,
			wantUnsupported: true,
		},
		{
			name: "future payload schema_version v2",
			raw: `{
				"specversion":"1.0","id":"1","source":"osac-metering","type":"osac.resource.heartbeat.v1",
				"time":"2026-10-01T12:00:00Z",
				"data":{"schema_version":"v2","tenant_id":"t1","resource_type":"volume","resource_id":"v1"}
			}`,
			wantUnsupported: true,
		},
		{
			name: "invalid JSON",
			raw:  `{not-json`,
		},
		{
			name: "unsupported resource_type",
			raw: `{
				"specversion":"1.0","id":"1","source":"osac-metering","type":"osac.resource.heartbeat.v1",
				"time":"2026-10-01T12:00:00Z",
				"data":{
					"tenant_id":"t1","resource_type":"unknown_thing","resource_id":"r1",
					"meter":"compute","component":"compute","heartbeat_time":"2026-10-01T12:05:00Z",
					"usage":{"semantics":"interval","from":"2026-10-01T12:00:00Z","to":"2026-10-01T12:05:00Z","quantity":"10","unit":"seconds","precision":"second"}
				}
			}`,
		},
		{
			name: "usage to before from",
			raw: `{
				"specversion":"1.0","id":"1","source":"osac-metering","type":"osac.resource.heartbeat.v1",
				"time":"2026-10-01T12:00:00Z",
				"data":{
					"tenant_id":"t1","resource_type":"volume","resource_id":"v1",
					"meter":"capacity","component":"volume","heartbeat_time":"2026-10-01T12:05:00Z",
					"usage":{"semantics":"interval","from":"2026-10-01T12:05:00Z","to":"2026-10-01T12:00:00Z","quantity":"10","unit":"gibibyte_second","precision":"second"}
				}
			}`,
		},
		{
			name: "non-decimal quantity",
			raw: `{
				"specversion":"1.0","id":"1","source":"osac-metering","type":"osac.resource.heartbeat.v1",
				"time":"2026-10-01T12:00:00Z",
				"data":{
					"tenant_id":"t1","resource_type":"volume","resource_id":"v1",
					"meter":"capacity","component":"volume","heartbeat_time":"2026-10-01T12:05:00Z",
					"usage":{"semantics":"interval","from":"2026-10-01T12:00:00Z","to":"2026-10-01T12:05:00Z","quantity":"NaN","unit":"gibibyte_second","precision":"second"}
				}
			}`,
		},
		{
			name: "extension tenant mismatch",
			raw: `{
				"specversion":"1.0","id":"1","source":"osac-metering","type":"osac.resource.heartbeat.v1",
				"time":"2026-10-01T12:00:00Z","osactenant":"other-tenant",
				"data":{
					"tenant_id":"t1","resource_type":"volume","resource_id":"v1",
					"meter":"capacity","component":"volume","heartbeat_time":"2026-10-01T12:05:00Z",
					"usage":{"semantics":"interval","from":"2026-10-01T12:00:00Z","to":"2026-10-01T12:05:00Z","quantity":"10","unit":"gibibyte_second","precision":"second"}
				}
			}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseRawEvent([]byte(tc.raw))
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !IsPermanentError(err) {
				t.Errorf("expected permanent error, got %T: %v", err, err)
			}
			var verErr *UnsupportedVersionError
			if tc.wantUnsupported && !errors.As(err, &verErr) {
				t.Errorf("expected UnsupportedVersionError, got %T: %v", err, err)
			}
		})
	}
}

func TestHasUsageObject_DetectsMalformedExplicitUsage(t *testing.T) {
	if !HasUsageObject(json.RawMessage(`{"usage":{"quantity":1200,"unit":"tokens"}}`)) {
		t.Error("expected malformed numeric usage.quantity to still be detected as explicit usage object")
	}
	if !HasUsageObject(json.RawMessage(`{"usage":"invalid"}`)) {
		t.Error("expected non-object explicit usage field to be detected")
	}
	if !HasUsageObject(json.RawMessage(`{"usage":{}}`)) {
		t.Error("expected empty explicit usage object to be detected")
	}
	if HasUsageObject(json.RawMessage(`{"usage":null}`)) {
		t.Error("expected explicit null usage to return false")
	}
	if HasUsageObject(json.RawMessage(`{"tenant_id":"t1"}`)) {
		t.Error("expected omitted usage to return false")
	}
}

func TestParseRawEvent_SchemaVersionPresenceAndQuantityBoundsAndSemantics(t *testing.T) {
	tests := []struct {
		name            string
		raw             string
		wantUnsupported bool
	}{
		{
			name: "explicit empty schema_version rejected",
			raw: `{
				"specversion":"1.0","id":"1","source":"osac-metering","type":"osac.resource.heartbeat.v1",
				"time":"2026-10-01T12:00:00Z",
				"data":{"schema_version":"","tenant_id":"t1","resource_type":"volume","resource_id":"v1","meter":"capacity","component":"volume","heartbeat_time":"2026-10-01T12:05:00Z","usage":{"semantics":"interval","from":"2026-10-01T12:00:00Z","to":"2026-10-01T12:05:00Z","quantity":"10","unit":"gibibyte_second","precision":"second"}}
			}`,
		},
		{
			name: "explicit null schema_version rejected",
			raw: `{
				"specversion":"1.0","id":"1","source":"osac-metering","type":"osac.resource.heartbeat.v1",
				"time":"2026-10-01T12:00:00Z",
				"data":{"schema_version":null,"tenant_id":"t1","resource_type":"volume","resource_id":"v1","meter":"capacity","component":"volume","heartbeat_time":"2026-10-01T12:05:00Z","usage":{"semantics":"interval","from":"2026-10-01T12:00:00Z","to":"2026-10-01T12:05:00Z","quantity":"10","unit":"gibibyte_second","precision":"second"}}
			}`,
		},
		{
			name: "quantity float overflow rejected (-1e309)",
			raw: `{
				"specversion":"1.0","id":"1","source":"osac-metering","type":"osac.resource.heartbeat.v1",
				"time":"2026-10-01T12:00:00Z",
				"data":{"tenant_id":"t1","resource_type":"volume","resource_id":"v1","meter":"capacity","component":"volume","heartbeat_time":"2026-10-01T12:05:00Z","usage":{"semantics":"interval","from":"2026-10-01T12:00:00Z","to":"2026-10-01T12:05:00Z","quantity":"-1e309","unit":"gibibyte_second","precision":"second"}}
			}`,
		},
		{
			name: "quantity NUMERIC(18,6) overflow rejected (1e13)",
			raw: `{
				"specversion":"1.0","id":"1","source":"osac-metering","type":"osac.resource.heartbeat.v1",
				"time":"2026-10-01T12:00:00Z",
				"data":{"tenant_id":"t1","resource_type":"volume","resource_id":"v1","meter":"capacity","component":"volume","heartbeat_time":"2026-10-01T12:05:00Z","usage":{"semantics":"interval","from":"2026-10-01T12:00:00Z","to":"2026-10-01T12:05:00Z","quantity":"1e13","unit":"gibibyte_second","precision":"second"}}
			}`,
		},
		{
			name: "nonzero quantity underflow rejected (-1e-400)",
			raw: `{
				"specversion":"1.0","id":"1","source":"osac-metering","type":"osac.resource.heartbeat.v1",
				"time":"2026-10-01T12:00:00Z",
				"data":{"tenant_id":"t1","resource_type":"volume","resource_id":"v1","meter":"capacity","component":"volume","heartbeat_time":"2026-10-01T12:05:00Z","usage":{"semantics":"interval","from":"2026-10-01T12:00:00Z","to":"2026-10-01T12:05:00Z","quantity":"-1e-400","unit":"gibibyte_second","precision":"second"}}
			}`,
		},
		{
			name: "inference cumulative semantics rejected",
			raw: `{
				"specversion":"1.0","id":"1","source":"osac-metering","type":"osac.inference.usage.v1",
				"time":"2026-10-01T12:00:00Z",
				"data":{"tenant_id":"t1","resource_type":"maas_inference","resource_id":"m1","usage":{"semantics":"cumulative","quantity":"1200","unit":"tokens"}}
			}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseRawEvent([]byte(tc.raw))
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !IsPermanentError(err) {
				t.Errorf("expected permanent error, got %T: %v", err, err)
			}
		})
	}

	// Verify explicit zero and fractional-second timestamp round-trip cleanly.
	validRaw := []byte(`{
		"specversion":"1.0","id":"frac-1","source":"osac-metering","type":"osac.resource.heartbeat.v1",
		"time":"2026-10-01T12:00:00.123456Z",
		"data":{"tenant_id":"t1","resource_type":"volume","resource_id":"v1","meter":"capacity","component":"volume","heartbeat_time":"2026-10-01T12:05:00Z","usage":{"semantics":"interval","from":"2026-10-01T12:00:00Z","to":"2026-10-01T12:05:00Z","quantity":"0.000000","unit":"gibibyte_second","precision":"second"}}
	}`)
	got, err := ParseRawEvent(validRaw)
	if err != nil {
		t.Fatalf("expected explicit zero + fractional time to succeed: %v", err)
	}
	if got.Envelope.Time != "2026-10-01T12:00:00.123456Z" {
		t.Errorf("Envelope.Time = %q, want 2026-10-01T12:00:00.123456Z", got.Envelope.Time)
	}
	if !got.Quantity.IsZero() {
		t.Errorf("Quantity = %s, want 0", got.Quantity)
	}
}
