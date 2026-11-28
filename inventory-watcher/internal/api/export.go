package api

import (
	"compress/gzip"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/osac-project/cost-event-consumer/internal/authn"
)

const maxExportDates = 366

// requirePlatformAdmin returns a forbidden error if the request carries an
// authenticated identity without IsPlatformAdmin. Unauthenticated requests
// (dev mode when AUTH_ISSUER_URL is empty) are allowed, matching enforceTenant.
func requirePlatformAdmin(ctx context.Context) error {
	id, ok := authn.IdentityFromContext(ctx)
	if !ok {
		return nil
	}
	if !id.IsPlatformAdmin {
		return fmt.Errorf("forbidden: platform administrator role required")
	}
	return nil
}

// enforceWalletTenant verifies that an authenticated non-platform-admin caller
// owns the target wallet before allowing reads or mutations on it.
func (h *APIHandler) enforceWalletTenant(ctx context.Context, walletID string) (int, error) {
	ident, ok := authn.IdentityFromContext(ctx)
	if !ok || ident.IsPlatformAdmin {
		return 0, nil
	}
	if ident.OrgID == "" {
		return http.StatusForbidden, fmt.Errorf("forbidden: identity has no organization")
	}
	if h.store == nil {
		return http.StatusForbidden, fmt.Errorf("forbidden: cannot verify wallet ownership")
	}
	wallet, err := h.store.GetWallet(ctx, walletID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return http.StatusNotFound, fmt.Errorf("wallet not found")
		}
		return http.StatusInternalServerError, fmt.Errorf("failed to verify wallet ownership")
	}
	if wallet.TenantID != ident.OrgID {
		return http.StatusForbidden, fmt.Errorf("forbidden: tenant mismatch")
	}
	return 0, nil
}

// GetExportDirtyDates implements ServerInterface (GET /api/v1/export/dirty-dates).
func (h *APIHandler) GetExportDirtyDates(w http.ResponseWriter, r *http.Request, params GetExportDirtyDatesParams) {
	w.Header().Set("Access-Control-Allow-Origin", "*")

	var tenantID string
	if params.TenantId != nil {
		tenantID = strings.TrimSpace(*params.TenantId)
	}
	if err := enforceTenant(r.Context(), &tenantID); err != nil {
		writeErrorJSON(w, err.Error(), http.StatusForbidden)
		return
	}

	var since *time.Time
	if params.Since != nil && strings.TrimSpace(*params.Since) != "" {
		raw := strings.TrimSpace(*params.Since)
		parsed, err := time.Parse(time.RFC3339Nano, raw)
		if err != nil {
			parsed, err = time.Parse(time.RFC3339, raw)
			if err != nil {
				writeErrorJSON(w, "invalid 'since' timestamp, use RFC3339", http.StatusBadRequest)
				return
			}
		}
		utc := parsed.UTC()
		since = &utc
	}

	res, err := h.store.QueryExportDirtyDates(r.Context(), since, tenantID)
	if err != nil {
		h.logger.Error("export dirty-dates query failed", "error", err)
		writeErrorJSON(w, "dirty-dates query failed", http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]any{
		"watermark": res.Watermark.UTC().Format(time.RFC3339Nano),
		"dates":     res.Dates,
	})
}

// GetExportDaily implements ServerInterface (GET /api/v1/export/daily).
func (h *APIHandler) GetExportDaily(w http.ResponseWriter, r *http.Request, params GetExportDailyParams) {
	w.Header().Set("Access-Control-Allow-Origin", "*")

	var tenantID string
	if params.TenantId != nil {
		tenantID = strings.TrimSpace(*params.TenantId)
	}
	if err := enforceTenant(r.Context(), &tenantID); err != nil {
		writeErrorJSON(w, err.Error(), http.StatusForbidden)
		return
	}

	dates, err := parseExportDates(params.Dates)
	if err != nil {
		writeErrorJSON(w, err.Error(), http.StatusBadRequest)
		return
	}

	format := resolveExportFormat(params.Format, r.Header.Get("Accept"))
	if format == "" {
		writeErrorJSON(w, "unsupported format: use ndjson, jsonl, csv, or json", http.StatusBadRequest)
		return
	}

	includeUserID := true
	if h.cfg != nil {
		includeUserID = h.cfg.ExportIncludeUserID
	}

	rows, err := h.store.QueryExportDailyRollups(r.Context(), dates, tenantID, includeUserID)
	if err != nil {
		h.logger.Error("export daily rollups query failed", "error", err)
		writeErrorJSON(w, "daily export query failed", http.StatusInternalServerError)
		return
	}

	var out io.Writer = w
	var gz *gzip.Writer
	if acceptsGzip(r.Header.Get("Accept-Encoding")) {
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Add("Vary", "Accept-Encoding")
		gz = gzip.NewWriter(w)
		defer func() { _ = gz.Close() }()
		out = gz
	}

	switch format {
	case "csv":
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", "attachment; filename=daily-cost-rollup.csv")
		cw := csv.NewWriter(out)
		_ = cw.Write([]string{
			"usage_date", "tenant_id", "tenant_name", "project_id", "project_name", "user_id",
			"service", "meter", "catalog_item", "instance_type", "resource_id", "resource_name",
			"cluster_id", "deployment", "cost_type", "usage_amount", "unit", "cost_amount",
			"currency", "tags", "entry_count",
		})
		for _, row := range rows {
			tagsBytes, _ := json.Marshal(row.Tags)
			_ = cw.Write([]string{
				row.UsageDate,
				csvFormulaSafe(row.TenantID),
				csvFormulaSafe(derefStr(row.TenantName)),
				csvFormulaSafe(derefStr(row.ProjectID)),
				csvFormulaSafe(derefStr(row.ProjectName)),
				csvFormulaSafe(derefStr(row.UserID)),
				csvFormulaSafe(row.Service),
				csvFormulaSafe(row.Meter),
				csvFormulaSafe(derefStr(row.CatalogItem)),
				csvFormulaSafe(derefStr(row.InstanceType)),
				csvFormulaSafe(derefStr(row.ResourceID)),
				csvFormulaSafe(derefStr(row.ResourceName)),
				csvFormulaSafe(derefStr(row.ClusterID)),
				csvFormulaSafe(derefStr(row.Deployment)),
				csvFormulaSafe(row.CostType),
				row.UsageAmount.String(),
				csvFormulaSafe(row.Unit),
				row.CostAmount.String(),
				csvFormulaSafe(row.Currency),
				string(tagsBytes),
				strconv.FormatInt(row.EntryCount, 10),
			})
		}
		cw.Flush()
	case "json":
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(out).Encode(map[string]any{
			"dates": dates,
			"count": len(rows),
			"data":  rows,
		})
	default: // "ndjson"
		w.Header().Set("Content-Type", "application/x-ndjson")
		enc := json.NewEncoder(out)
		for _, row := range rows {
			if err := enc.Encode(row); err != nil {
				return
			}
		}
	}
}

func parseExportDates(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("dates parameter is required (comma-separated YYYY-MM-DD)")
	}
	parts := strings.Split(raw, ",")
	seen := make(map[string]struct{}, len(parts))
	dates := make([]string, 0, len(parts))
	for _, p := range parts {
		d := strings.TrimSpace(p)
		if d == "" {
			continue
		}
		parsed, err := time.Parse("2006-01-02", d)
		if err != nil || parsed.Format("2006-01-02") != d {
			return nil, fmt.Errorf("invalid date %q: expected YYYY-MM-DD", d)
		}
		if _, ok := seen[d]; !ok {
			seen[d] = struct{}{}
			dates = append(dates, d)
		}
	}
	if len(dates) == 0 {
		return nil, fmt.Errorf("dates parameter must contain at least one YYYY-MM-DD date")
	}
	if len(dates) > maxExportDates {
		return nil, fmt.Errorf("too many dates requested (%d): maximum is %d", len(dates), maxExportDates)
	}
	return dates, nil
}

func resolveExportFormat(param *GetExportDailyParamsFormat, accept string) string {
	if param != nil && *param != "" {
		switch strings.ToLower(string(*param)) {
		case "ndjson", "jsonl":
			return "ndjson"
		case "csv":
			return "csv"
		case "json":
			return "json"
		default:
			return ""
		}
	}
	lowerAccept := strings.ToLower(accept)
	switch {
	case strings.Contains(lowerAccept, "text/csv"):
		return "csv"
	case strings.Contains(lowerAccept, "application/x-ndjson"), strings.Contains(lowerAccept, "application/jsonl"):
		return "ndjson"
	case strings.Contains(lowerAccept, "application/json"):
		return "json"
	default:
		return "ndjson"
	}
}

func acceptsGzip(acceptEncoding string) bool {
	for _, part := range strings.Split(acceptEncoding, ",") {
		enc := strings.TrimSpace(strings.Split(part, ";")[0])
		if strings.EqualFold(enc, "gzip") {
			return true
		}
	}
	return false
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func csvFormulaSafe(s string) string {
	if len(s) > 0 && (s[0] == '=' || s[0] == '+' || s[0] == '-' || s[0] == '@') {
		return "'" + s
	}
	return s
}
