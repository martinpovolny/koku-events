// Package osacmetering owns the locked Cost Management v1 ingestion contract
// and the explicit translation boundary from OSAC metering v1 CloudEvents.
//
// Both integration paths — HTTP delivery from the OSAC Cost Management adapter
// (which already emits the locked CloudEventEnvelope) and direct Kafka
// consumption via OSAC metering.readers (which delivers raw OSAC
// osac.resource.heartbeat.v1 CloudEvents) — pass through this package so OSAC
// schema evolution and internal pipeline fields cannot leak past the boundary.
package osacmetering

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/osac-project/cost-event-consumer/internal/inventory"
)

const (
	// ContractVersion identifies the locked Cost Management v1 usage payload schema.
	ContractVersion = "v1"

	// CloudEventVersionV1 is the required CloudEvents specification version.
	CloudEventVersionV1 = "1.0"

	// ApplicationJSON is the expected CloudEvents datacontenttype.
	ApplicationJSON = "application/json"

	// EventTypeHeartbeat is the canonical OSAC metering v1 interval usage heartbeat.
	EventTypeHeartbeat = "osac.resource.heartbeat.v1"

	// EventTypeInferenceUsage is the canonical OSAC metering v1 MaaS inference usage event.
	EventTypeInferenceUsage = "osac.inference.usage.v1"

	// UsageSemanticsInterval indicates an additive usage quantity covering [from, to].
	UsageSemanticsInterval = "interval"

	// UsageSemanticsDelta indicates a per-event usage delta (used by inference events).
	UsageSemanticsDelta = "delta"

	// MaxIDLength mirrors the ingest handler limit for resource and tenant IDs.
	MaxIDLength = 256

	// Supported OSAC metering v1 resource types.
	ResourceTypeComputeInstance   = "compute_instance"
	ResourceTypeClusterOrder      = "cluster_order"
	ResourceTypeBareMetalInstance = "bare_metal_instance"
	ResourceTypeExternalIP        = "external_ip"
	ResourceTypeNATGateway        = "nat_gateway"
	ResourceTypeVolume            = "volume"
	ResourceTypeMaaSInference     = "maas_inference"
)

// CloudEventEnvelope is the locked CloudEvents 1.0 structured-mode wire
// contract shared between the OSAC Cost Management adapter and koku-events.
type CloudEventEnvelope struct {
	SpecVersion     string    `json:"specversion"`
	ID              string    `json:"id"`
	Source          string    `json:"source"`
	Type            string    `json:"type"`
	DataContentType string    `json:"datacontenttype"`
	Time            string    `json:"time"`
	Subject         string    `json:"subject,omitempty"`
	ResourceID      string    `json:"osacresourceid"`
	ResourceType    string    `json:"osacresourcetype"`
	Tenant          string    `json:"osactenant"`
	Project         string    `json:"osacproject,omitempty"`
	Data            UsageData `json:"data"`
}

// UsageData is the locked Cost Management v1 usage payload inside
// CloudEventEnvelope. OSAC-internal bookkeeping fields (such as
// last_applied_offset and rules_revision) are intentionally excluded.
type UsageData struct {
	SchemaVersion            string            `json:"schema_version"`
	TenantID                 string            `json:"tenant_id"`
	ExternalBillingAccountID string            `json:"external_billing_account_id,omitempty"`
	ProjectID                string            `json:"project_id,omitempty"`
	ResourceType             string            `json:"resource_type"`
	ResourceID               string            `json:"resource_id"`
	Meter                    string            `json:"meter"`
	Component                string            `json:"component"`
	HeartbeatTime            string            `json:"heartbeat_time,omitempty"`
	Usage                    UsageInterval     `json:"usage"`
	BillingDimensions        map[string]string `json:"billing_dimensions,omitempty"`
}

// UsageInterval carries one measured usage slice in Cost Management's v1
// contract. Quantity is fixed-point decimal text; negative quantities represent
// additive billing corrections to prior usage.
type UsageInterval struct {
	Semantics string `json:"semantics"`
	From      string `json:"from,omitempty"`
	To        string `json:"to,omitempty"`
	Quantity  string `json:"quantity"`
	Unit      string `json:"unit"`
	Precision string `json:"precision,omitempty"`
}

// NormalizedUsage is a validated, normalized Cost Management v1 usage event
// with parsed timestamps and exact decimal quantity ready for ingestion.
type NormalizedUsage struct {
	Envelope      CloudEventEnvelope
	EventTime     time.Time
	HeartbeatTime time.Time
	PeriodStart   time.Time
	PeriodEnd     time.Time
	Quantity      decimal.Decimal
}

// EffectiveTenantID returns the billing tenant identifier for this usage
// record. When OSAC populates external_billing_account_id, it takes precedence
// over the internal tenant_id for billing attribution.
func (d UsageData) EffectiveTenantID() string {
	if acct := strings.TrimSpace(d.ExternalBillingAccountID); acct != "" {
		return acct
	}
	return d.TenantID
}

// EffectiveProjectID returns the project identifier from project_id or the
// billing_dimensions["project"] identity dimension.
func (d UsageData) EffectiveProjectID() string {
	if d.ProjectID != "" {
		return d.ProjectID
	}
	return d.BillingDimensions["project"]
}

// InstanceType returns the machine/instance sizing dimension if present.
func (d UsageData) InstanceType() string {
	if v := d.BillingDimensions["instance_type"]; v != "" {
		return v
	}
	return d.BillingDimensions["host_type"]
}

// CatalogItem returns the catalog or template dimension if present.
func (d UsageData) CatalogItem() string {
	if v := d.BillingDimensions["catalog"]; v != "" {
		return v
	}
	if v := d.BillingDimensions["catalog_item"]; v != "" {
		return v
	}
	return d.BillingDimensions["template"]
}

// MeterName returns the canonical Cost Management meter name for this event.
func (d UsageData) MeterName() string {
	if d.Meter != "" {
		return d.Meter
	}
	if d.Component != "" {
		return d.Component
	}
	return "inference"
}

// UserID returns the end-user identifier dimension if present (D15).
func (d UsageData) UserID() string {
	if v := d.BillingDimensions["user_id"]; v != "" {
		return v
	}
	return d.BillingDimensions["user"]
}

// Deployment returns the deployment/region placement dimension if present (G2).
func (d UsageData) Deployment() string {
	return d.BillingDimensions["deployment"]
}

// ClusterID returns the associated cluster identifier for cluster orders or
// cluster-hosted compute resources (G4).
func (d UsageData) ClusterID() string {
	if v := d.BillingDimensions["cluster_id"]; v != "" {
		return v
	}
	if d.ResourceType == ResourceTypeClusterOrder {
		return d.ResourceID
	}
	return ""
}

// BillingDimensionsJSON serializes BillingDimensions as a JSON object suitable
// for JSONB storage on metering_entries.
func (d UsageData) BillingDimensionsJSON() json.RawMessage {
	if len(d.BillingDimensions) == 0 {
		return json.RawMessage(`{}`)
	}
	b, err := json.Marshal(d.BillingDimensions)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return json.RawMessage(b)
}

// ToMeteringEntry maps a normalized OSAC v1 usage event into a Cost Management
// metering_entries row. Negative quantities are preserved as signed values so
// additive corrections adjust period totals and rating output.
func (n *NormalizedUsage) ToMeteringEntry() inventory.MeteringEntry {
	return inventory.MeteringEntry{
		ResourceType:      n.Envelope.Data.ResourceType,
		ResourceID:        n.Envelope.Data.ResourceID,
		TenantID:          n.Envelope.Data.EffectiveTenantID(),
		ProjectID:         n.Envelope.Data.EffectiveProjectID(),
		UserID:            n.Envelope.Data.UserID(),
		InstanceType:      n.Envelope.Data.InstanceType(),
		CatalogItem:       n.Envelope.Data.CatalogItem(),
		Deployment:        n.Envelope.Data.Deployment(),
		ClusterID:         n.Envelope.Data.ClusterID(),
		BillingDimensions: n.Envelope.Data.BillingDimensionsJSON(),
		MeterName:         n.Envelope.Data.MeterName(),
		Value:             n.Quantity.InexactFloat64(),
		Unit:              n.Envelope.Data.Usage.Unit,
		PeriodStart:       n.PeriodStart,
		PeriodEnd:         n.PeriodEnd,
	}
}

// ValidationError signals a permanent schema, identity, or field validation
// error that cannot succeed on retry.
type ValidationError struct {
	Message string
	Err     error
}

func (e *ValidationError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *ValidationError) Unwrap() error { return e.Err }

// DeadLetterReason implements kafka.PermanentError without importing kafka.
func (e *ValidationError) DeadLetterReason() string { return "validation_error" }

// UnsupportedVersionError signals an unrecognized OSAC metering event or
// payload schema version (for example osac.resource.heartbeat.v2).
type UnsupportedVersionError struct {
	Message string
}

func (e *UnsupportedVersionError) Error() string { return e.Message }

// DeadLetterReason implements kafka.PermanentError without importing kafka.
func (e *UnsupportedVersionError) DeadLetterReason() string { return "unsupported_version" }

// IsPermanentError reports whether err represents a permanent contract or
// validation error that should not be retried.
func IsPermanentError(err error) bool {
	var valErr *ValidationError
	var verErr *UnsupportedVersionError
	return errors.As(err, &valErr) || errors.As(err, &verErr)
}
