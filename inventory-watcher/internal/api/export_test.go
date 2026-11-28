package api_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/osac-project/cost-event-consumer/internal/api"
	"github.com/osac-project/cost-event-consumer/internal/authn"
	"github.com/osac-project/cost-event-consumer/internal/config"
	"github.com/osac-project/cost-event-consumer/internal/inventory"
	"github.com/osac-project/cost-event-consumer/internal/rating"
)

func TestExportDirtyDatesAndDailyRollup_EndToEnd(t *testing.T) {
	ctx := context.Background()
	tenant := fmt.Sprintf("export-tenant-%d", time.Now().UnixNano())
	otherTenant := fmt.Sprintf("export-other-%d", time.Now().UnixNano())

	// Ensure a deterministic rate for compute_instance/compute and a custom service.
	rateID, err := testStore.UpsertRate(ctx, inventory.RateRecord{
		TenantID:      &tenant,
		ResourceType:  "compute_instance",
		MeterName:     "compute",
		CostType:      "Infrastructure",
		PricePerUnit:  decimal.RequireFromString("0.01"),
		Currency:      "USD",
		EffectiveFrom: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("upsert rate: %v", err)
	}
	defer func() { _, _ = testStore.DeleteRate(ctx, rateID) }()

	customRateID, err := testStore.UpsertRate(ctx, inventory.RateRecord{
		TenantID:      &tenant,
		ResourceType:  "custom_gpu_job",
		MeterName:     "gpu_seconds",
		CostType:      "Supplementary",
		PricePerUnit:  decimal.RequireFromString("0.05"),
		Currency:      "USD",
		EffectiveFrom: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("upsert custom rate: %v", err)
	}
	defer func() { _, _ = testStore.DeleteRate(ctx, customRateID) }()

	// Seed optional inventory records for OSAC enrichment check.
	if err := testStore.UpsertTenant(ctx, inventory.TenantRecord{
		TenantID:  tenant,
		Name:      "Acme Export Tenant",
		Labels:    json.RawMessage(`{"cost_center":"cc-42"}`),
		CreatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("upsert tenant: %v", err)
	}
	if err := testStore.UpsertProject(ctx, inventory.ProjectRecord{
		ProjectID: "proj-alpha",
		Name:      "Alpha Project",
		Tenant:    tenant,
		Labels:    json.RawMessage(`{"team":"finops"}`),
		CreatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("upsert project: %v", err)
	}

	// Day 1: 2026-06-15 UTC — two users on same compute_instance + a negative backdated correction.
	day1 := time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC)
	day1End := day1.Add(time.Hour)
	day2 := time.Date(2026, 6, 16, 14, 0, 0, 0, time.UTC)
	day2End := day2.Add(time.Hour)

	batch := []map[string]any{
		{
			"specversion":      "1.0",
			"id":               fmt.Sprintf("exp-hb-1-%d", time.Now().UnixNano()),
			"source":           "osac-metering",
			"type":             "osac.resource.heartbeat.v1",
			"datacontenttype":  "application/json",
			"time":             day1.Format(time.RFC3339),
			"osacresourceid":   "vm-exp-1",
			"osacresourcetype": "compute_instance",
			"osactenant":       tenant,
			"osacproject":      "proj-alpha",
			"data": map[string]any{
				"schema_version": "v1",
				"tenant_id":      tenant,
				"project_id":     "proj-alpha",
				"resource_type":  "compute_instance",
				"resource_id":    "vm-exp-1",
				"meter":          "compute",
				"component":      "compute",
				"heartbeat_time": time.Now().UTC().Format(time.RFC3339),
				"usage": map[string]any{
					"semantics": "interval",
					"from":      day1.Format(time.RFC3339),
					"to":        day1End.Format(time.RFC3339),
					"quantity":  "100.000000",
					"unit":      "seconds",
					"precision": "second",
				},
				"billing_dimensions": map[string]string{
					"instance_type": "standard-4-8",
					"catalog":       "osac.vm.medium",
					"deployment":    "eu-central-1",
					"cluster_id":    "ocp-cluster-7",
					"user_id":       "alice",
					"env":           "prod",
				},
			},
		},
		{
			"specversion":      "1.0",
			"id":               fmt.Sprintf("exp-hb-2-%d", time.Now().UnixNano()),
			"source":           "osac-metering",
			"type":             "osac.resource.heartbeat.v1",
			"datacontenttype":  "application/json",
			"time":             day1.Format(time.RFC3339),
			"osacresourceid":   "vm-exp-1",
			"osacresourcetype": "compute_instance",
			"osactenant":       tenant,
			"osacproject":      "proj-alpha",
			"data": map[string]any{
				"schema_version": "v1",
				"tenant_id":      tenant,
				"project_id":     "proj-alpha",
				"resource_type":  "compute_instance",
				"resource_id":    "vm-exp-1",
				"meter":          "compute",
				"component":      "compute",
				"heartbeat_time": time.Now().UTC().Format(time.RFC3339),
				"usage": map[string]any{
					"semantics": "interval",
					"from":      day1.Format(time.RFC3339),
					"to":        day1End.Format(time.RFC3339),
					"quantity":  "50.000000",
					"unit":      "seconds",
					"precision": "second",
				},
				"billing_dimensions": map[string]string{
					"instance_type": "standard-4-8",
					"catalog":       "osac.vm.medium",
					"deployment":    "eu-central-1",
					"cluster_id":    "ocp-cluster-7",
					"user_id":       "bob",
					"env":           "prod",
				},
			},
		},
	}

	rawBatch, _ := json.Marshal(map[string]any{"events": batch})
	resp, err := http.Post(testServer.URL+"/api/v1/events/batch", "application/json", bytes.NewReader(rawBatch))
	if err != nil {
		t.Fatalf("ingest batch: %v", err)
	}
	respBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204 from batch ingest, got %d: %s", resp.StatusCode, string(respBody))
	}

	// Also insert an OSAC-less custom resource metering entry (no inventory_* rows).
	if err := testStore.InsertMeteringEntry(ctx, inventory.MeteringEntry{
		ResourceType:      "custom_gpu_job",
		ResourceID:        "job-99",
		TenantID:          tenant,
		MeterName:         "gpu_seconds",
		Value:             200,
		Unit:              "gpu_seconds",
		Deployment:        "us-east-1",
		BillingDimensions: json.RawMessage(`{"app":"training"}`),
		PeriodStart:       day2,
		PeriodEnd:         day2End,
	}); err != nil {
		t.Fatalf("insert custom metering entry: %v", err)
	}

	// Also insert a metering entry for otherTenant on day1 to verify tenant scoping.
	if err := testStore.InsertMeteringEntry(ctx, inventory.MeteringEntry{
		ResourceType: "compute_instance",
		ResourceID:   "vm-other-1",
		TenantID:     otherTenant,
		MeterName:    "compute",
		Value:        10,
		Unit:         "seconds",
		PeriodStart:  day1,
		PeriodEnd:    day1End,
	}); err != nil {
		t.Fatalf("insert other tenant metering entry: %v", err)
	}

	// Rate all pending metering entries.
	rater := rating.New(testStore, time.Minute, 500, testLogger)
	rater.SweepOnce(ctx)

	// 1. Initial dirty-dates discovery (no `since` watermark) must return 2026-06-15, 2026-06-16, plus {yesterday, today}.
	ddResp, err := http.Get(fmt.Sprintf("%s/api/v1/export/dirty-dates?tenant_id=%s", testServer.URL, tenant))
	if err != nil {
		t.Fatalf("GET dirty-dates: %v", err)
	}
	defer ddResp.Body.Close()
	if ddResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from dirty-dates, got %d", ddResp.StatusCode)
	}
	var ddBody struct {
		Watermark string   `json:"watermark"`
		Dates     []string `json:"dates"`
	}
	if err := json.NewDecoder(ddResp.Body).Decode(&ddBody); err != nil {
		t.Fatalf("decode dirty-dates: %v", err)
	}
	if ddBody.Watermark == "" {
		t.Fatal("expected non-empty watermark")
	}
	dateSet := make(map[string]bool, len(ddBody.Dates))
	for _, d := range ddBody.Dates {
		dateSet[d] = true
	}
	todayUTC := time.Now().UTC().Format("2006-01-02")
	yesterdayUTC := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")
	for _, want := range []string{"2026-06-15", "2026-06-16", yesterdayUTC, todayUTC} {
		if !dateSet[want] {
			t.Errorf("expected dirty dates %v to contain %s", ddBody.Dates, want)
		}
	}

	// 2. Subsequent dirty-dates with `since=watermark` should NOT include 2026-06-15 or 2026-06-16 (only safety floor).
	ddResp2, err := http.Get(fmt.Sprintf("%s/api/v1/export/dirty-dates?tenant_id=%s&since=%s", testServer.URL, tenant, ddBody.Watermark))
	if err != nil {
		t.Fatalf("GET dirty-dates with since: %v", err)
	}
	defer ddResp2.Body.Close()
	var ddBody2 struct {
		Watermark string   `json:"watermark"`
		Dates     []string `json:"dates"`
	}
	if err := json.NewDecoder(ddResp2.Body).Decode(&ddBody2); err != nil {
		t.Fatalf("decode dirty-dates 2: %v", err)
	}
	for _, d := range ddBody2.Dates {
		if d == "2026-06-15" || d == "2026-06-16" {
			t.Errorf("did not expect unchanged historical date %s after watermark", d)
		}
	}

	// 3. Ingest a late-arriving NEGATIVE correction for 2026-06-15 (-30 seconds for alice) and re-rate.
	// Wait a tiny tick so created_at is strictly after ddBody.Watermark.
	time.Sleep(5 * time.Millisecond)
	corrBatch := []map[string]any{
		{
			"specversion":      "1.0",
			"id":               fmt.Sprintf("exp-hb-corr-%d", time.Now().UnixNano()),
			"source":           "osac-metering",
			"type":             "osac.resource.heartbeat.v1",
			"datacontenttype":  "application/json",
			"time":             day1.Format(time.RFC3339),
			"osacresourceid":   "vm-exp-1",
			"osacresourcetype": "compute_instance",
			"osactenant":       tenant,
			"osacproject":      "proj-alpha",
			"data": map[string]any{
				"schema_version": "v1",
				"tenant_id":      tenant,
				"project_id":     "proj-alpha",
				"resource_type":  "compute_instance",
				"resource_id":    "vm-exp-1",
				"meter":          "compute",
				"component":      "compute",
				"heartbeat_time": time.Now().UTC().Format(time.RFC3339),
				"usage": map[string]any{
					"semantics": "interval",
					"from":      day1.Format(time.RFC3339),
					"to":        day1End.Format(time.RFC3339),
					"quantity":  "-30.000000",
					"unit":      "seconds",
					"precision": "second",
				},
				"billing_dimensions": map[string]string{
					"instance_type": "standard-4-8",
					"catalog":       "osac.vm.medium",
					"deployment":    "eu-central-1",
					"cluster_id":    "ocp-cluster-7",
					"user_id":       "alice",
					"env":           "prod",
				},
			},
		},
	}
	corrRaw, _ := json.Marshal(map[string]any{"events": corrBatch})
	corrResp, err := http.Post(testServer.URL+"/api/v1/events/batch", "application/json", bytes.NewReader(corrRaw))
	if err != nil {
		t.Fatalf("ingest correction: %v", err)
	}
	corrBody, _ := io.ReadAll(corrResp.Body)
	corrResp.Body.Close()
	if corrResp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204 for correction, got %d: %s", corrResp.StatusCode, string(corrBody))
	}
	rater.SweepOnce(ctx)

	// Dirty-dates with previous watermark MUST rediscover 2026-06-15 (and NOT 2026-06-16)!
	ddResp3, err := http.Get(fmt.Sprintf("%s/api/v1/export/dirty-dates?tenant_id=%s&since=%s", testServer.URL, tenant, ddBody.Watermark))
	if err != nil {
		t.Fatalf("GET dirty-dates after correction: %v", err)
	}
	defer ddResp3.Body.Close()
	var ddBody3 struct {
		Watermark string   `json:"watermark"`
		Dates     []string `json:"dates"`
	}
	if err := json.NewDecoder(ddResp3.Body).Decode(&ddBody3); err != nil {
		t.Fatalf("decode dirty-dates 3: %v", err)
	}
	foundDay1 := false
	for _, d := range ddBody3.Dates {
		if d == "2026-06-15" {
			foundDay1 = true
		}
		if d == "2026-06-16" {
			t.Errorf("unchanged date 2026-06-16 should not be marked dirty")
		}
	}
	if !foundDay1 {
		t.Fatalf("expected backdated correction to mark 2026-06-15 dirty; got %v", ddBody3.Dates)
	}

	// 4. Fetch GET /api/v1/export/daily in gzipped NDJSON (default format) for both dates.
	req, _ := http.NewRequest(http.MethodGet,
		fmt.Sprintf("%s/api/v1/export/daily?dates=2026-06-15,2026-06-16&tenant_id=%s", testServer.URL, tenant), nil)
	req.Header.Set("Accept-Encoding", "gzip")
	expResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET export daily: %v", err)
	}
	defer expResp.Body.Close()
	if expResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", expResp.StatusCode)
	}
	if expResp.Header.Get("Content-Encoding") != "gzip" {
		t.Fatalf("expected Content-Encoding: gzip, got %q", expResp.Header.Get("Content-Encoding"))
	}
	gzReader, err := gzip.NewReader(expResp.Body)
	if err != nil {
		t.Fatalf("gzip reader: %v", err)
	}
	defer gzReader.Close()
	rawNDJSON, err := io.ReadAll(gzReader)
	if err != nil {
		t.Fatalf("read gzipped ndjson: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(rawNDJSON)), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 rollup rows (alice day1, bob day1, custom job day2), got %d:\n%s", len(lines), string(rawNDJSON))
	}

	var rows []inventory.DailyCostRollupRow
	for _, line := range lines {
		var r inventory.DailyCostRollupRow
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			t.Fatalf("unmarshal rollup line %q: %v", line, err)
		}
		rows = append(rows, r)
	}

	// Check alice's net signed row (100 - 30 = 70 seconds * 0.01 = 0.70 USD, entry_count=2).
	var aliceRow, customRow *inventory.DailyCostRollupRow
	for i := range rows {
		if rows[i].UserID != nil && *rows[i].UserID == "alice" {
			aliceRow = &rows[i]
		}
		if rows[i].Service == "custom_gpu_job" {
			customRow = &rows[i]
		}
	}
	if aliceRow == nil {
		t.Fatalf("missing alice row in %+v", rows)
	}
	if !aliceRow.UsageAmount.Equal(decimal.RequireFromString("70")) {
		t.Errorf("alice net usage_amount = %s, want 70", aliceRow.UsageAmount)
	}
	if !aliceRow.CostAmount.Equal(decimal.RequireFromString("0.70")) {
		t.Errorf("alice net cost_amount = %s, want 0.70", aliceRow.CostAmount)
	}
	if aliceRow.EntryCount != 2 {
		t.Errorf("alice entry_count = %d, want 2", aliceRow.EntryCount)
	}
	if aliceRow.TenantName == nil || *aliceRow.TenantName != "Acme Export Tenant" {
		t.Errorf("unexpected tenant_name: %v", aliceRow.TenantName)
	}
	if aliceRow.ProjectName == nil || *aliceRow.ProjectName != "Alpha Project" {
		t.Errorf("unexpected project_name: %v", aliceRow.ProjectName)
	}
	if aliceRow.Deployment == nil || *aliceRow.Deployment != "eu-central-1" {
		t.Errorf("unexpected deployment: %v", aliceRow.Deployment)
	}
	if aliceRow.ClusterID == nil || *aliceRow.ClusterID != "ocp-cluster-7" {
		t.Errorf("unexpected cluster_id: %v", aliceRow.ClusterID)
	}
	if aliceRow.Tags["cost_center"] != "cc-42" || aliceRow.Tags["team"] != "finops" || aliceRow.Tags["env"] != "prod" {
		t.Errorf("unexpected merged tags: %+v", aliceRow.Tags)
	}

	// Check generic OSAC-less row (custom_gpu_job, no inventory rows, Supplementary cost type).
	if customRow == nil {
		t.Fatalf("missing custom_gpu_job row in %+v", rows)
	}
	if customRow.CostType != "Supplementary" || !customRow.CostAmount.Equal(decimal.RequireFromString("10")) {
		t.Errorf("unexpected custom row cost_type=%s cost_amount=%s", customRow.CostType, customRow.CostAmount)
	}
	if customRow.Tags["app"] != "training" || customRow.Tags["cost_center"] != "cc-42" {
		t.Errorf("unexpected custom row tags: %+v", customRow.Tags)
	}

	// 5. Verify CSV export format preserves signed decimals cleanly.
	csvResp, err := http.Get(fmt.Sprintf("%s/api/v1/export/daily?dates=2026-06-15&tenant_id=%s&format=csv", testServer.URL, tenant))
	if err != nil {
		t.Fatalf("GET export daily csv: %v", err)
	}
	defer csvResp.Body.Close()
	if csvResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for csv, got %d", csvResp.StatusCode)
	}
	csvRows, err := csv.NewReader(csvResp.Body).ReadAll()
	if err != nil {
		t.Fatalf("parse csv: %v", err)
	}
	if len(csvRows) != 3 { // header + alice + bob
		t.Fatalf("expected 3 CSV rows (header + 2 users), got %d", len(csvRows))
	}

	// 6. Verify EXPORT_INCLUDE_USER_ID=false collapses per-user rows (and strips user_id/user tags) (D15).
	noUserCfg := &config.Config{ExportIncludeUserID: false}
	noUserHandler := api.NewAPIHandler(testStore, testMeter, noUserCfg, nil, testLogger)
	noUserMux := http.NewServeMux()
	api.HandlerFromMux(noUserHandler, noUserMux)

	w := httptest.NewRecorder()
	collapseReq := httptest.NewRequest(http.MethodGet,
		fmt.Sprintf("/api/v1/export/daily?dates=2026-06-15&tenant_id=%s&format=json", tenant), nil)
	noUserMux.ServeHTTP(w, collapseReq)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var collapsed struct {
		Count int                            `json:"count"`
		Data  []inventory.DailyCostRollupRow `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &collapsed); err != nil {
		t.Fatalf("unmarshal collapsed json: %v", err)
	}
	if collapsed.Count != 1 {
		t.Fatalf("expected 1 collapsed row for 2026-06-15 when EXPORT_INCLUDE_USER_ID=false, got %d: %+v", collapsed.Count, collapsed.Data)
	}
	if collapsed.Data[0].UserID != nil {
		t.Errorf("expected nil user_id when collapsed, got %q", *collapsed.Data[0].UserID)
	}
	if _, leaked := collapsed.Data[0].Tags["user_id"]; leaked {
		t.Errorf("expected user_id stripped from tags when EXPORT_INCLUDE_USER_ID=false, got %+v", collapsed.Data[0].Tags)
	}
	// Net usage for 2026-06-15: alice (100 - 30 = 70) + bob (50) = 120 seconds, cost = 1.20 USD, entry_count = 3.
	if !collapsed.Data[0].UsageAmount.Equal(decimal.RequireFromString("120")) ||
		!collapsed.Data[0].CostAmount.Equal(decimal.RequireFromString("1.20")) ||
		collapsed.Data[0].EntryCount != 3 {
		t.Errorf("unexpected collapsed totals: usage=%s cost=%s entries=%d",
			collapsed.Data[0].UsageAmount, collapsed.Data[0].CostAmount, collapsed.Data[0].EntryCount)
	}
}

func TestO3_TenantAndAdminAuthorization_DB(t *testing.T) {
	ctx := context.Background()
	tenantA := fmt.Sprintf("authz-a-%d", time.Now().UnixNano())
	tenantB := fmt.Sprintf("authz-b-%d", time.Now().UnixNano())

	walletA := inventory.WalletRecord{
		ID:             fmt.Sprintf("wallet-a-%d", time.Now().UnixNano()),
		TenantID:       tenantA,
		Currency:       "USD",
		LifecycleState: "active",
	}
	if err := testStore.CreateWallet(ctx, walletA); err != nil {
		t.Fatalf("create walletA: %v", err)
	}

	rateB, err := testStore.UpsertRate(ctx, inventory.RateRecord{
		TenantID:      &tenantB,
		ResourceType:  "compute_instance",
		MeterName:     "private_meter_b",
		PricePerUnit:  decimal.RequireFromString("1.0"),
		Currency:      "USD",
		EffectiveFrom: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("create tenantB rate: %v", err)
	}
	defer func() { _, _ = testStore.DeleteRate(ctx, rateB) }()

	h := api.NewAPIHandler(testStore, testMeter, &config.Config{}, nil, testLogger)
	mux := http.NewServeMux()
	api.HandlerFromMux(h, mux)

	asTenantB := func(req *http.Request) *http.Request {
		return req.WithContext(authn.WithIdentity(req.Context(), &authn.Identity{
			OrgID:      tenantB,
			Username:   "bob",
			IsOrgAdmin: true,
		}))
	}

	// Cross-tenant wallet reads & mutations must return 403.
	t.Run("cross-tenant GetWalletStatus returns 403", func(t *testing.T) {
		req := asTenantB(httptest.NewRequest(http.MethodGet, "/api/v1/wallets/"+walletA.ID, nil))
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d (%s)", w.Code, w.Body.String())
		}
	})

	t.Run("cross-tenant TopUpWallet returns 403", func(t *testing.T) {
		req := asTenantB(httptest.NewRequest(http.MethodPost, "/api/v1/wallets/"+walletA.ID+"/top-ups",
			strings.NewReader(`{"amount":"50.00"}`)))
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d (%s)", w.Code, w.Body.String())
		}
	})

	t.Run("cross-tenant AdjustWallet returns 403", func(t *testing.T) {
		req := asTenantB(httptest.NewRequest(http.MethodPost, "/api/v1/wallets/"+walletA.ID+"/adjustments",
			strings.NewReader(`{"amount":"10.00"}`)))
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d (%s)", w.Code, w.Body.String())
		}
	})

	t.Run("cross-tenant GetWalletLedger returns 403", func(t *testing.T) {
		req := asTenantB(httptest.NewRequest(http.MethodGet, "/api/v1/wallets/"+walletA.ID+"/ledger", nil))
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d (%s)", w.Code, w.Body.String())
		}
	})

	// Tenant A listing rates without tenant_id must NOT see Tenant B's private rate.
	t.Run("ListRates hides other tenant private rates", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/rates", nil)
		req = req.WithContext(authn.WithIdentity(req.Context(), &authn.Identity{
			OrgID:    tenantA,
			Username: "alice",
		}))
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		if strings.Contains(w.Body.String(), "private_meter_b") {
			t.Fatalf("tenant A should not see tenant B private rate: %s", w.Body.String())
		}
	})
}
