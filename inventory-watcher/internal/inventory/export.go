package inventory

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
)

// QueryExportDirtyDates discovers all UTC usage dates that have cost_entries
// created after since (or all dates when since is nil), unioned with the
// {yesterday_utc, today_utc} safety floor, inside a single REPEATABLE READ
// read-only snapshot (G1 / D9).
func (s *Store) QueryExportDirtyDates(ctx context.Context, since *time.Time, tenantID string) (*ExportDirtyDatesResult, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{
		IsoLevel:   pgx.RepeatableRead,
		AccessMode: pgx.ReadOnly,
	})
	if err != nil {
		return nil, fmt.Errorf("begin export dirty-dates tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var maxCreatedAt *time.Time
	var dbNow time.Time
	if err := tx.QueryRow(ctx, `
		SELECT MAX(created_at), NOW()
		FROM cost_entries
		WHERE ($1 = '' OR tenant_id = $1)
	`, tenantID).Scan(&maxCreatedAt, &dbNow); err != nil {
		return nil, fmt.Errorf("query export watermark: %w", err)
	}

	watermark := dbNow.UTC()
	if maxCreatedAt != nil {
		watermark = maxCreatedAt.UTC()
	} else if since != nil {
		watermark = since.UTC()
	}
	if since != nil && since.UTC().After(watermark) {
		watermark = since.UTC()
	}

	rows, err := tx.Query(ctx, `
		SELECT DISTINCT (period_start AT TIME ZONE 'UTC')::date::text
		FROM cost_entries
		WHERE ($1::timestamptz IS NULL OR created_at > $1)
		  AND ($2 = '' OR tenant_id = $2)
		ORDER BY 1
	`, since, tenantID)
	if err != nil {
		return nil, fmt.Errorf("query dirty dates: %w", err)
	}
	defer rows.Close()

	dateSet := make(map[string]struct{})
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			return nil, fmt.Errorf("scan dirty date: %w", err)
		}
		if d != "" {
			dateSet[d] = struct{}{}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate dirty dates: %w", err)
	}

	nowUTC := dbNow.UTC()
	dateSet[nowUTC.AddDate(0, 0, -1).Format("2006-01-02")] = struct{}{}
	dateSet[nowUTC.Format("2006-01-02")] = struct{}{}

	dates := make([]string, 0, len(dateSet))
	for d := range dateSet {
		dates = append(dates, d)
	}
	sort.Strings(dates)

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit export dirty-dates tx: %w", err)
	}

	return &ExportDirtyDatesResult{
		Watermark: watermark,
		Dates:     dates,
	}, nil
}

// QueryExportDailyRollups returns complete UTC-day rollup rows from
// v1_daily_cost_rollup for the requested dates, optionally filtered by
// tenantID. When includeUserID is false (EXPORT_INCLUDE_USER_ID=false, D15),
// per-user rows are collapsed with user_id set to NULL.
func (s *Store) QueryExportDailyRollups(ctx context.Context, dates []string, tenantID string, includeUserID bool) ([]DailyCostRollupRow, error) {
	if len(dates) == 0 {
		return []DailyCostRollupRow{}, nil
	}

	var query string
	if includeUserID {
		query = `
			SELECT usage_date::text, tenant_id, tenant_name, project_id, project_name, user_id,
			       service, meter, catalog_item, instance_type, resource_id, resource_name,
			       cluster_id, deployment, cost_type, usage_amount, unit, cost_amount,
			       currency, tags, entry_count
			FROM v1_daily_cost_rollup
			WHERE usage_date = ANY($1::date[])
			  AND ($2 = '' OR tenant_id = $2)
			ORDER BY usage_date, tenant_id, service, meter, resource_id NULLS FIRST, user_id NULLS FIRST
		`
	} else {
		query = `
			SELECT usage_date::text, tenant_id, tenant_name, project_id, project_name, NULL::text AS user_id,
			       service, meter, catalog_item, instance_type, resource_id, resource_name,
			       cluster_id, deployment, cost_type, SUM(usage_amount) AS usage_amount, unit,
			       SUM(cost_amount) AS cost_amount, currency, (tags - 'user_id' - 'user') AS tags,
			       SUM(entry_count)::bigint AS entry_count
			FROM v1_daily_cost_rollup
			WHERE usage_date = ANY($1::date[])
			  AND ($2 = '' OR tenant_id = $2)
			GROUP BY usage_date, tenant_id, tenant_name, project_id, project_name,
			         service, meter, catalog_item, instance_type, resource_id, resource_name,
			         cluster_id, deployment, cost_type, unit, currency, (tags - 'user_id' - 'user')
			ORDER BY usage_date, tenant_id, service, meter, resource_id NULLS FIRST
		`
	}

	rows, err := s.db.Query(ctx, query, dates, tenantID)
	if err != nil {
		return nil, fmt.Errorf("query daily cost rollups: %w", err)
	}
	defer rows.Close()

	results := make([]DailyCostRollupRow, 0)
	for rows.Next() {
		var r DailyCostRollupRow
		var tagsRaw []byte
		if err := rows.Scan(
			&r.UsageDate, &r.TenantID, &r.TenantName, &r.ProjectID, &r.ProjectName, &r.UserID,
			&r.Service, &r.Meter, &r.CatalogItem, &r.InstanceType, &r.ResourceID, &r.ResourceName,
			&r.ClusterID, &r.Deployment, &r.CostType, &r.UsageAmount, &r.Unit, &r.CostAmount,
			&r.Currency, &tagsRaw, &r.EntryCount,
		); err != nil {
			return nil, fmt.Errorf("scan daily cost rollup: %w", err)
		}
		r.Tags = decodeTagsMap(tagsRaw)
		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate daily cost rollups: %w", err)
	}
	return results, nil
}

func decodeTagsMap(raw []byte) map[string]string {
	tags := make(map[string]string)
	if len(raw) == 0 {
		return tags
	}
	var generic map[string]any
	if err := json.Unmarshal(raw, &generic); err != nil {
		return tags
	}
	for k, v := range generic {
		if v == nil {
			continue
		}
		switch typed := v.(type) {
		case string:
			tags[k] = typed
		default:
			if b, err := json.Marshal(typed); err == nil {
				tags[k] = string(b)
			} else {
				tags[k] = fmt.Sprint(typed)
			}
		}
	}
	return tags
}
