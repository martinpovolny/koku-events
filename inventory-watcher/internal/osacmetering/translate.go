package osacmetering

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

// maxAbsQuantity is the largest absolute magnitude representable in
// PostgreSQL metering_entries.value NUMERIC(18,6) (< 10^12).
var maxAbsQuantity = decimal.RequireFromString("999999999999.999999")

var supportedHeartbeatResourceTypes = map[string]struct{}{
	ResourceTypeComputeInstance:   {},
	ResourceTypeClusterOrder:      {},
	ResourceTypeBareMetalInstance: {},
	ResourceTypeExternalIP:        {},
	ResourceTypeNATGateway:        {},
	ResourceTypeVolume:            {},
}

// EnvelopeInput is the raw structured CloudEvents 1.0 envelope decoded from
// either HTTP or Kafka prior to contract normalization.
type EnvelopeInput struct {
	SpecVersion      string          `json:"specversion"`
	ID               string          `json:"id"`
	Source           string          `json:"source"`
	Type             string          `json:"type"`
	DataContentType  string          `json:"datacontenttype"`
	Time             time.Time       `json:"time"`
	Subject          string          `json:"subject,omitempty"`
	OSACResourceID   string          `json:"osacresourceid,omitempty"`
	OSACResourceType string          `json:"osacresourcetype,omitempty"`
	OSACTenant       string          `json:"osactenant,omitempty"`
	OSACProject      string          `json:"osacproject,omitempty"`
	Data             json.RawMessage `json:"data"`
}

type rawUsagePayload struct {
	SchemaVersion            json.RawMessage `json:"schema_version"`
	TenantID                 string          `json:"tenant_id"`
	ExternalBillingAccountID *string         `json:"external_billing_account_id,omitempty"`
	ProjectID                *string         `json:"project_id,omitempty"`
	ResourceType             string          `json:"resource_type"`
	ResourceID               string          `json:"resource_id"`
	Meter                    string          `json:"meter"`
	Component                string          `json:"component"`
	HeartbeatTime            string          `json:"heartbeat_time"`
	Usage                    *rawUsageObject `json:"usage"`
	BillingDimensions        map[string]any  `json:"billing_dimensions"`
}

type rawUsageObject struct {
	Semantics string `json:"semantics"`
	From      string `json:"from"`
	To        string `json:"to"`
	Quantity  string `json:"quantity"`
	Unit      string `json:"unit"`
	Precision string `json:"precision"`
}

// IsUnsupportedOSACVersion reports whether an OSAC metering heartbeat or
// inference event type carries an unsupported major version suffix (such as
// osac.resource.heartbeat.v2).
func IsUnsupportedOSACVersion(eventType string) bool {
	if strings.HasPrefix(eventType, "osac.resource.heartbeat.") && eventType != EventTypeHeartbeat {
		return true
	}
	if strings.HasPrefix(eventType, "osac.inference.usage.") && eventType != EventTypeInferenceUsage {
		return true
	}
	return false
}

// HasUsageObject reports whether a CloudEvent data payload explicitly includes
// a non-null usage field (distinguishing canonical interval/delta usage events
// — including malformed ones that must be rejected by the v1 translator — from
// legacy lifecycle/inference events that omit usage or set it to null).
func HasUsageObject(rawData json.RawMessage) bool {
	if len(rawData) == 0 || bytes.Equal(bytes.TrimSpace(rawData), []byte("null")) {
		return false
	}
	var peek struct {
		Usage json.RawMessage `json:"usage"`
	}
	if err := json.Unmarshal(rawData, &peek); err != nil {
		return false
	}
	trimmed := bytes.TrimSpace(peek.Usage)
	return len(trimmed) > 0 && !bytes.Equal(trimmed, []byte("null"))
}

// ParseRawEvent decodes a raw structured CloudEvent JSON payload and translates
// it into the locked Cost Management v1 contract.
func ParseRawEvent(raw []byte) (*NormalizedUsage, error) {
	var in EnvelopeInput
	if err := json.Unmarshal(raw, &in); err != nil {
		return nil, &ValidationError{Message: "invalid CloudEvent JSON", Err: err}
	}
	return TranslateEnvelope(in)
}

// TranslateEnvelope validates an incoming CloudEvent envelope and maps both raw
// OSAC v1 events (from Kafka metering.readers) and adapter-mapped locked
// CloudEventEnvelopes (from HTTP) into NormalizedUsage.
func TranslateEnvelope(in EnvelopeInput) (*NormalizedUsage, error) {
	if err := validateEnvelope(in); err != nil {
		return nil, err
	}
	if IsUnsupportedOSACVersion(in.Type) {
		return nil, &UnsupportedVersionError{
			Message: fmt.Sprintf("unsupported OSAC metering CloudEvent version %q", in.Type),
		}
	}

	switch in.Type {
	case EventTypeHeartbeat:
		return translateHeartbeat(in)
	case EventTypeInferenceUsage:
		return translateInference(in)
	default:
		return nil, &ValidationError{
			Message: fmt.Sprintf("unsupported OSAC metering CloudEvent type %q", in.Type),
		}
	}
}

func validateEnvelope(in EnvelopeInput) error {
	if in.SpecVersion != CloudEventVersionV1 {
		return &ValidationError{Message: fmt.Sprintf("specversion must be %s, got %q", CloudEventVersionV1, in.SpecVersion)}
	}
	if in.ID == "" || in.Source == "" || in.Type == "" || in.Time.IsZero() {
		return &ValidationError{Message: "specversion, id, type, source, and time are required"}
	}
	if in.DataContentType != "" && in.DataContentType != ApplicationJSON {
		return &ValidationError{Message: fmt.Sprintf("datacontenttype must be %s, got %q", ApplicationJSON, in.DataContentType)}
	}
	if len(in.Data) == 0 || string(in.Data) == "null" {
		return &ValidationError{Message: "event data is required"}
	}
	return nil
}

func translateHeartbeat(in EnvelopeInput) (*NormalizedUsage, error) {
	var data rawUsagePayload
	if err := json.Unmarshal(in.Data, &data); err != nil {
		return nil, &ValidationError{Message: "invalid OSAC heartbeat data", Err: err}
	}
	if err := validateOptionalSchemaVersion(data.SchemaVersion); err != nil {
		return nil, err
	}
	if err := validateIdentity(in, data); err != nil {
		return nil, err
	}
	if _, ok := supportedHeartbeatResourceTypes[data.ResourceType]; !ok {
		return nil, &ValidationError{Message: fmt.Sprintf("unsupported heartbeat resource_type %q", data.ResourceType)}
	}
	if data.Meter == "" || data.Component == "" {
		return nil, &ValidationError{Message: "heartbeat data missing required meter or component"}
	}
	if data.HeartbeatTime == "" {
		return nil, &ValidationError{Message: "heartbeat data missing heartbeat_time"}
	}
	hbTime, err := time.Parse(time.RFC3339Nano, data.HeartbeatTime)
	if err != nil {
		return nil, &ValidationError{Message: fmt.Sprintf("invalid heartbeat_time %q", data.HeartbeatTime), Err: err}
	}
	if data.Usage == nil {
		return nil, &ValidationError{Message: "heartbeat data missing usage object"}
	}
	fromTime, toTime, qty, err := validateIntervalUsage(*data.Usage)
	if err != nil {
		return nil, err
	}

	dims := normalizeDimensions(data.BillingDimensions)
	project := resolveProject(data.ProjectID, in.OSACProject, dims)
	billingAcct := trimPtr(data.ExternalBillingAccountID)

	env := CloudEventEnvelope{
		SpecVersion:     CloudEventVersionV1,
		ID:              in.ID,
		Source:          in.Source,
		Type:            in.Type,
		DataContentType: ApplicationJSON,
		Time:            in.Time.UTC().Format(time.RFC3339Nano),
		Subject:         in.Subject,
		ResourceID:      data.ResourceID,
		ResourceType:    data.ResourceType,
		Tenant:          data.TenantID,
		Project:         project,
		Data: UsageData{
			SchemaVersion:            ContractVersion,
			TenantID:                 data.TenantID,
			ExternalBillingAccountID: billingAcct,
			ProjectID:                project,
			ResourceType:             data.ResourceType,
			ResourceID:               data.ResourceID,
			Meter:                    data.Meter,
			Component:                data.Component,
			HeartbeatTime:            data.HeartbeatTime,
			Usage: UsageInterval{
				Semantics: data.Usage.Semantics,
				From:      data.Usage.From,
				To:        data.Usage.To,
				Quantity:  data.Usage.Quantity,
				Unit:      data.Usage.Unit,
				Precision: data.Usage.Precision,
			},
			BillingDimensions: dims,
		},
	}

	return &NormalizedUsage{
		Envelope:      env,
		EventTime:     in.Time.UTC(),
		HeartbeatTime: hbTime.UTC(),
		PeriodStart:   fromTime.UTC(),
		PeriodEnd:     toTime.UTC(),
		Quantity:      qty,
	}, nil
}

func translateInference(in EnvelopeInput) (*NormalizedUsage, error) {
	var data rawUsagePayload
	if err := json.Unmarshal(in.Data, &data); err != nil {
		return nil, &ValidationError{Message: "invalid OSAC inference data", Err: err}
	}
	if err := validateOptionalSchemaVersion(data.SchemaVersion); err != nil {
		return nil, err
	}
	if err := validateIdentity(in, data); err != nil {
		return nil, err
	}
	if data.ResourceType != ResourceTypeMaaSInference {
		return nil, &ValidationError{
			Message: fmt.Sprintf("unexpected inference resource_type %q, want %q", data.ResourceType, ResourceTypeMaaSInference),
		}
	}
	if data.Usage == nil {
		return nil, &ValidationError{Message: "inference data missing usage object"}
	}
	qty, err := validateQuantityAndUnit(*data.Usage)
	if err != nil {
		return nil, err
	}

	semantics := data.Usage.Semantics
	if semantics == "" {
		semantics = UsageSemanticsDelta
	} else if semantics != UsageSemanticsDelta && semantics != UsageSemanticsInterval {
		return nil, &ValidationError{
			Message: fmt.Sprintf("unsupported inference usage.semantics %q, want %q or %q", semantics, UsageSemanticsDelta, UsageSemanticsInterval),
		}
	}

	periodStart := in.Time.UTC()
	periodEnd := in.Time.UTC()
	if data.Usage.From != "" || data.Usage.To != "" {
		if data.Usage.From == "" || data.Usage.To == "" {
			return nil, &ValidationError{Message: "inference usage must specify both from and to when interval bounds are present"}
		}
		fromTime, err := time.Parse(time.RFC3339Nano, data.Usage.From)
		if err != nil {
			return nil, &ValidationError{Message: fmt.Sprintf("invalid usage.from %q", data.Usage.From), Err: err}
		}
		toTime, err := time.Parse(time.RFC3339Nano, data.Usage.To)
		if err != nil {
			return nil, &ValidationError{Message: fmt.Sprintf("invalid usage.to %q", data.Usage.To), Err: err}
		}
		if toTime.Before(fromTime) {
			return nil, &ValidationError{Message: fmt.Sprintf("usage.to %q precedes usage.from %q", data.Usage.To, data.Usage.From)}
		}
		periodStart = fromTime.UTC()
		periodEnd = toTime.UTC()
	}

	dims := normalizeDimensions(data.BillingDimensions)
	project := resolveProject(data.ProjectID, in.OSACProject, dims)
	billingAcct := trimPtr(data.ExternalBillingAccountID)

	meter := data.Meter
	if meter == "" {
		meter = "inference"
	}
	component := data.Component
	if component == "" {
		component = meter
	}

	env := CloudEventEnvelope{
		SpecVersion:     CloudEventVersionV1,
		ID:              in.ID,
		Source:          in.Source,
		Type:            in.Type,
		DataContentType: ApplicationJSON,
		Time:            in.Time.UTC().Format(time.RFC3339Nano),
		Subject:         in.Subject,
		ResourceID:      data.ResourceID,
		ResourceType:    data.ResourceType,
		Tenant:          data.TenantID,
		Project:         project,
		Data: UsageData{
			SchemaVersion:            ContractVersion,
			TenantID:                 data.TenantID,
			ExternalBillingAccountID: billingAcct,
			ProjectID:                project,
			ResourceType:             data.ResourceType,
			ResourceID:               data.ResourceID,
			Meter:                    meter,
			Component:                component,
			Usage: UsageInterval{
				Semantics: semantics,
				From:      data.Usage.From,
				To:        data.Usage.To,
				Quantity:  data.Usage.Quantity,
				Unit:      data.Usage.Unit,
				Precision: data.Usage.Precision,
			},
			BillingDimensions: dims,
		},
	}

	return &NormalizedUsage{
		Envelope:    env,
		EventTime:   in.Time.UTC(),
		PeriodStart: periodStart,
		PeriodEnd:   periodEnd,
		Quantity:    qty,
	}, nil
}

func validateIdentity(in EnvelopeInput, data rawUsagePayload) error {
	if data.TenantID == "" || data.ResourceType == "" || data.ResourceID == "" {
		return &ValidationError{Message: "usage data missing required identity fields (tenant_id, resource_type, resource_id)"}
	}
	if len(data.ResourceID) > MaxIDLength || len(data.TenantID) > MaxIDLength {
		return &ValidationError{Message: "resource_id or tenant_id exceeds maximum length"}
	}
	if billingAcct := trimPtr(data.ExternalBillingAccountID); len(billingAcct) > MaxIDLength {
		return &ValidationError{Message: "external_billing_account_id exceeds maximum length"}
	}
	if in.OSACResourceID != "" && in.OSACResourceID != data.ResourceID {
		return &ValidationError{Message: fmt.Sprintf("extension osacresourceid %q does not match data.resource_id %q", in.OSACResourceID, data.ResourceID)}
	}
	if in.OSACResourceType != "" && in.OSACResourceType != data.ResourceType {
		return &ValidationError{Message: fmt.Sprintf("extension osacresourcetype %q does not match data.resource_type %q", in.OSACResourceType, data.ResourceType)}
	}
	if in.OSACTenant != "" && in.OSACTenant != data.TenantID {
		return &ValidationError{Message: fmt.Sprintf("extension osactenant %q does not match data.tenant_id %q", in.OSACTenant, data.TenantID)}
	}
	return nil
}

func validateIntervalUsage(u rawUsageObject) (time.Time, time.Time, decimal.Decimal, error) {
	if u.Semantics != UsageSemanticsInterval {
		return time.Time{}, time.Time{}, decimal.Zero, &ValidationError{
			Message: fmt.Sprintf("heartbeat usage.semantics must be %q, got %q", UsageSemanticsInterval, u.Semantics),
		}
	}
	if u.From == "" || u.To == "" {
		return time.Time{}, time.Time{}, decimal.Zero, &ValidationError{
			Message: "heartbeat usage missing from or to timestamp",
		}
	}
	fromTime, err := time.Parse(time.RFC3339Nano, u.From)
	if err != nil {
		return time.Time{}, time.Time{}, decimal.Zero, &ValidationError{
			Message: fmt.Sprintf("invalid usage.from %q", u.From), Err: err,
		}
	}
	toTime, err := time.Parse(time.RFC3339Nano, u.To)
	if err != nil {
		return time.Time{}, time.Time{}, decimal.Zero, &ValidationError{
			Message: fmt.Sprintf("invalid usage.to %q", u.To), Err: err,
		}
	}
	if toTime.Before(fromTime) {
		return time.Time{}, time.Time{}, decimal.Zero, &ValidationError{
			Message: fmt.Sprintf("usage.to %q precedes usage.from %q", u.To, u.From),
		}
	}
	qty, err := validateQuantityAndUnit(u)
	if err != nil {
		return time.Time{}, time.Time{}, decimal.Zero, err
	}
	if u.Precision == "" {
		return time.Time{}, time.Time{}, decimal.Zero, &ValidationError{
			Message: "heartbeat usage missing precision",
		}
	}
	return fromTime, toTime, qty, nil
}

func validateOptionalSchemaVersion(raw json.RawMessage) error {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		// Field omitted: valid for raw OSAC v1 Kafka events.
		return nil
	}
	if bytes.Equal(trimmed, []byte("null")) {
		return &ValidationError{Message: "schema_version must be a non-empty string when present"}
	}
	var version string
	if err := json.Unmarshal(trimmed, &version); err != nil {
		return &ValidationError{Message: "schema_version must be a string", Err: err}
	}
	if version == "" {
		return &ValidationError{Message: "schema_version must not be empty when present"}
	}
	if version != ContractVersion {
		return &UnsupportedVersionError{
			Message: fmt.Sprintf("unsupported OSAC usage schema_version %q, want %q", version, ContractVersion),
		}
	}
	return nil
}

func validateQuantityAndUnit(u rawUsageObject) (decimal.Decimal, error) {
	if u.Quantity == "" {
		return decimal.Zero, &ValidationError{Message: "usage missing quantity"}
	}
	qty, err := decimal.NewFromString(u.Quantity)
	if err != nil {
		return decimal.Zero, &ValidationError{Message: fmt.Sprintf("invalid decimal usage.quantity %q", u.Quantity), Err: err}
	}
	if qty.Abs().GreaterThan(maxAbsQuantity) {
		return decimal.Zero, &ValidationError{Message: fmt.Sprintf("usage.quantity %q exceeds supported NUMERIC(18,6) range", u.Quantity)}
	}
	f := qty.InexactFloat64()
	if math.IsInf(f, 0) || math.IsNaN(f) {
		return decimal.Zero, &ValidationError{Message: fmt.Sprintf("usage.quantity %q is not a finite number", u.Quantity)}
	}
	if !qty.IsZero() && (qty.Round(6).IsZero() || f == 0) {
		return decimal.Zero, &ValidationError{Message: fmt.Sprintf("usage.quantity %q underflows supported precision", u.Quantity)}
	}
	if u.Unit == "" {
		return decimal.Zero, &ValidationError{Message: "usage missing unit"}
	}
	return qty, nil
}

func resolveProject(dataProject *string, extProject string, dims map[string]string) string {
	if dataProject != nil && strings.TrimSpace(*dataProject) != "" {
		return strings.TrimSpace(*dataProject)
	}
	if extProject != "" {
		return extProject
	}
	return dims["project"]
}

func trimPtr(s *string) string {
	if s == nil {
		return ""
	}
	return strings.TrimSpace(*s)
}

func normalizeDimensions(in map[string]any) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, raw := range in {
		if k == "" || raw == nil {
			continue
		}
		switch v := raw.(type) {
		case string:
			if v != "" {
				out[k] = v
			}
		case bool:
			out[k] = strconv.FormatBool(v)
		case float64:
			if v != 0 {
				out[k] = strconv.FormatFloat(v, 'f', -1, 64)
			}
		default:
			if s := fmt.Sprint(v); s != "" {
				out[k] = s
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
