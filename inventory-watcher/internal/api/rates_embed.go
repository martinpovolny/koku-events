package api

const ratesHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Cost Management — Catalog &amp; Rates</title>
<style>
  :root {
    --bg: #f0f4f8; --card: #fff; --border: #dde3ea; --text: #1a1a2e;
    --muted: #6b7280; --accent: #cc0000; --accent2: #a30000;
    --green: #16a34a; --amber: #d97706; --red: #dc2626; --blue: #1d4ed8;
    --purple: #7c3aed; --teal: #0f766e;
  }
  * { box-sizing: border-box; margin: 0; padding: 0; }
  body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
         background: var(--bg); color: var(--text); min-height: 100vh; }

  /* Header */
  .header { background: var(--accent); color: white; padding: 0 2rem;
            display: flex; align-items: center; justify-content: space-between; height: 56px; }
  .header-left { display: flex; align-items: center; gap: 1rem; }
  .header-logo { font-size: 1.15rem; font-weight: 700; letter-spacing: -0.02em; }
  .header-sub { font-size: 0.82rem; opacity: 0.85; padding-left: 1rem; border-left: 1px solid rgba(255,255,255,0.3); }
  .header-nav { display: flex; align-items: center; gap: 0.75rem; margin-left: 1.5rem; }
  .header-nav a { color: rgba(255,255,255,0.85); text-decoration: none; font-size: 0.82rem; padding: 0.25rem 0.5rem; border-radius: 4px; }
  .header-nav a:hover { background: rgba(255,255,255,0.15); color: white; }
  .header-nav a.active { background: rgba(0,0,0,0.2); font-weight: 600; color: white; }
  .header-right { display: flex; align-items: center; gap: 0.75rem; }
  .token-btn { padding: 0.3rem 0.8rem; border-radius: 4px;
               border: 1px solid rgba(255,255,255,0.4); background: transparent;
               color: white; cursor: pointer; font-size: 0.82rem; }
  .token-btn.has-token { border-color: #86efac; color: #86efac; }

  /* Toolbar */
  .toolbar { background: white; border-bottom: 1px solid var(--border);
             padding: 0 2rem; display: flex; align-items: center; gap: 1rem;
             height: 48px; font-size: 0.85rem; color: var(--muted); flex-wrap: wrap; }
  .toolbar label { display: flex; align-items: center; gap: 0.4rem; }
  .toolbar input, .toolbar select {
    border: 1px solid var(--border); border-radius: 4px;
    padding: 0.25rem 0.6rem; font-size: 0.85rem; color: var(--text);
    background: white; }
  .toolbar .spacer { flex: 1; }
  .btn-sm { padding: 0.35rem 0.8rem; border-radius: 5px; font-size: 0.82rem;
            font-weight: 500; cursor: pointer; border: 1px solid var(--border); background: white; color: var(--text); }
  .btn-sm:hover { background: #f9fafb; }
  .btn-sm.primary { background: var(--accent); color: white; border-color: var(--accent); }
  .btn-sm.primary:hover { background: var(--accent2); }
  .btn-xs { padding: 0.2rem 0.5rem; font-size: 0.75rem; border-radius: 4px; border: 1px solid var(--border); cursor: pointer; background: white; }
  .btn-xs:hover { background: #f3f4f6; }

  /* KPI tiles */
  .kpi-row { display: grid; grid-template-columns: repeat(4, 1fr); gap: 1rem;
             padding: 1.5rem 2rem 0; max-width: 1400px; margin: 0 auto; }
  .kpi { background: white; border-radius: 8px; padding: 1.25rem 1.5rem;
         border: 1px solid var(--border); box-shadow: 0 1px 3px rgba(0,0,0,0.05); }
  .kpi-value { font-size: 2rem; font-weight: 700; line-height: 1; }
  .kpi-label { font-size: 0.78rem; color: var(--muted); text-transform: uppercase;
               letter-spacing: 0.06em; margin-top: 0.4rem; }
  .kpi.instances .kpi-value { color: var(--blue); }
  .kpi.catalog .kpi-value { color: var(--purple); }
  .kpi.rates .kpi-value { color: var(--green); }
  .kpi.rules .kpi-value { color: var(--teal); }

  /* Main content */
  .content { padding: 1.5rem 2rem; max-width: 1400px; margin: 0 auto; }

  /* Tabs */
  .tabs { display: flex; gap: 0; margin-bottom: 1.25rem; border-bottom: 2px solid var(--border); }
  .tab { padding: 0.6rem 1.25rem; cursor: pointer; font-size: 0.88rem;
         color: var(--muted); border-bottom: 2px solid transparent; margin-bottom: -2px; }
  .tab.active { color: var(--accent); border-bottom-color: var(--accent); font-weight: 600; }
  .tab:hover:not(.active) { color: var(--text); }

  /* Cards and tables */
  .card { background: white; border: 1px solid var(--border); border-radius: 8px;
          box-shadow: 0 1px 3px rgba(0,0,0,0.05); overflow: hidden; margin-bottom: 1.5rem; }
  .card-header { padding: 1rem 1.25rem; border-bottom: 1px solid var(--border);
                 font-weight: 600; font-size: 0.9rem; display: flex;
                 align-items: center; justify-content: space-between; }
  .card-header .subtitle { font-weight: 400; color: var(--muted); font-size: 0.82rem; }
  table { width: 100%; border-collapse: collapse; font-size: 0.87rem; }
  th { background: #fafbfc; padding: 0.65rem 1.25rem; text-align: left;
       font-size: 0.76rem; text-transform: uppercase; letter-spacing: 0.05em;
       color: var(--muted); border-bottom: 1px solid var(--border); font-weight: 600; }
  td { padding: 0.65rem 1.25rem; border-bottom: 1px solid #f3f4f6; vertical-align: middle; }
  tr:last-child td { border-bottom: none; }
  tr:hover td { background: #fafbfc; }
  td.num, th.num { text-align: right; font-family: "SF Mono", "Fira Code", monospace; }
  .empty { text-align: center; padding: 3rem; color: var(--muted); font-size: 0.9rem; }

  /* Badges */
  .badge { display: inline-block; padding: 0.18rem 0.55rem; border-radius: 9999px;
           font-size: 0.72rem; font-weight: 600; }
  .badge-inst { background: #dbeafe; color: var(--blue); }
  .badge-cat { background: #ede9fe; color: var(--purple); }
  .badge-bm { background: #ccfbf1; color: var(--teal); }
  .badge-priced { background: #dcfce7; color: var(--green); }
  .badge-unpriced { background: #fef3c7; color: var(--amber); }
  .badge-infra { background: #e0e7ff; color: #3730a3; }
  .badge-supp { background: #f3e8ff; color: #6b21a8; }
  .badge-gorule { background: #ccfbf1; color: #0f766e; }

  .code-text { font-family: "SF Mono", "Fira Code", monospace; font-size: 0.8rem; background: #f3f4f6; padding: 0.15rem 0.35rem; border-radius: 3px; }

  /* Alerts */
  .error-banner { background: #fee2e2; color: var(--red); padding: 0.75rem 1.25rem;
                  border-radius: 6px; margin-bottom: 1rem; font-size: 0.88rem; display: none; }
  .success-banner { background: #dcfce7; color: var(--green); padding: 0.75rem 1.25rem;
                    border-radius: 6px; margin-bottom: 1rem; font-size: 0.88rem; display: none; }

  /* Modals */
  .modal-overlay { position: fixed; inset: 0; background: rgba(0,0,0,0.45);
                   display: flex; align-items: center; justify-content: center; z-index: 100; }
  .modal { background: white; border-radius: 10px; padding: 1.75rem; width: 620px;
           max-width: 95vw; max-height: 90vh; overflow-y: auto; box-shadow: 0 20px 60px rgba(0,0,0,0.2); }
  .modal.wide { width: 800px; }
  .modal h2 { font-size: 1.15rem; margin-bottom: 0.4rem; color: var(--text); }
  .modal p { font-size: 0.85rem; color: var(--muted); margin-bottom: 1rem; line-height: 1.5; }
  .form-group { margin-bottom: 1rem; }
  .form-group label { display: block; font-size: 0.8rem; font-weight: 600; color: var(--text); margin-bottom: 0.3rem; }
  .form-group input, .form-group select, .form-group textarea {
    width: 100%; border: 1px solid var(--border); border-radius: 5px;
    padding: 0.5rem 0.75rem; font-size: 0.88rem; color: var(--text); background: white; }
  .form-group .hint { font-size: 0.75rem; color: var(--muted); margin-top: 0.25rem; }
  .form-row { display: grid; grid-template-columns: 1fr 1fr; gap: 0.75rem; }
  .model-selector { display: grid; grid-template-columns: repeat(3, 1fr); gap: 0.5rem; margin-bottom: 1rem; }
  .model-card { border: 2px solid var(--border); border-radius: 6px; padding: 0.6rem; cursor: pointer; text-align: center; }
  .model-card.active { border-color: var(--accent); background: #fff5f5; font-weight: 600; }
  .model-card .title { font-size: 0.82rem; }
  .model-card .sub { font-size: 0.7rem; color: var(--muted); margin-top: 0.2rem; }

  .preview-box { background: #f8fafc; border: 1px solid var(--border); border-radius: 5px; padding: 0.6rem 0.8rem; font-size: 0.78rem; font-family: monospace; max-height: 120px; overflow-y: auto; margin-top: 0.35rem; }
  .modal-btns { display: flex; gap: 0.5rem; justify-content: flex-end; margin-top: 1.25rem; }
  .modal-btns button { padding: 0.5rem 1.2rem; border-radius: 6px; border: none;
                       cursor: pointer; font-size: 0.88rem; font-weight: 500; }
  .btn-primary { background: var(--accent); color: white; }
  .btn-primary:hover { background: var(--accent2); }
  .btn-secondary { background: #f0f4f8; color: var(--text); }
  .btn-danger { background: var(--red); color: white; }
  .btn-danger:hover { background: #b91c1c; }
</style>
</head>
<body>

<div class="header">
  <div class="header-left">
    <span class="header-logo">Cost Management</span>
    <span class="header-sub">Catalog &amp; Rates</span>
    <nav class="header-nav">
      <a href="/ui">Overview</a>
      <a href="/ui/rates" class="active">Catalog &amp; Rates</a>
      <a href="/ui/reports">Reports</a>
      <a href="/ui/dashboard">Diagnostics</a>
    </nav>
  </div>
  <div class="header-right">
    <span id="lastUpdated" style="font-size:0.78rem;opacity:0.85"></span>
    <button class="token-btn" id="tokenBtn" onclick="showTokenModal()">Token</button>
  </div>
</div>

<div class="toolbar">
  <label>Search <input type="text" id="filterSearch" placeholder="Filter by SKU, name, meter…" oninput="renderAll()"></label>
  <label>Category
    <select id="filterType" onchange="renderAll()">
      <option value="">All Categories</option>
      <option value="instance_type">Machine Instance Types</option>
      <option value="compute_instance">Compute Offerings</option>
      <option value="cluster">Cluster Offerings</option>
      <option value="bare_metal">Bare Metal Offerings</option>
      <option value="model">Model / AI Offerings</option>
    </select>
  </label>
  <label>Pricing
    <select id="filterPricing" onchange="renderAll()">
      <option value="">All Items</option>
      <option value="priced">Priced Only</option>
      <option value="unpriced">Unpriced Only</option>
    </select>
  </label>
  <div class="spacer"></div>
  <button class="btn-sm primary" onclick="openAssignModal()">+ Assign Rate</button>
  <button class="btn-sm" onclick="openCreateRuleModal()">+ New GoRule</button>
  <button class="btn-sm" onclick="downloadCSV()">Export CSV</button>
  <button class="btn-sm" onclick="loadAll()">↻ Refresh</button>
</div>

<div class="kpi-row">
  <div class="kpi instances">
    <div class="kpi-value" id="kInstances">—</div>
    <div class="kpi-label">Machine Sizing Types</div>
  </div>
  <div class="kpi catalog">
    <div class="kpi-value" id="kCatalog">—</div>
    <div class="kpi-label">Service Catalog Items</div>
  </div>
  <div class="kpi rates">
    <div class="kpi-value" id="kRates">—</div>
    <div class="kpi-label">Active Rate Cards</div>
  </div>
  <div class="kpi rules">
    <div class="kpi-value" id="kRules">—</div>
    <div class="kpi-label">GoRules Decision Models</div>
  </div>
</div>

<div class="content">
  <div id="errorBanner" class="error-banner"></div>
  <div id="successBanner" class="success-banner"></div>

  <div class="tabs">
    <div class="tab active" data-tab="catalog" onclick="switchTab(this)">Catalog &amp; Rate Assignment</div>
    <div class="tab" data-tab="rates" onclick="switchTab(this)">Active Rate Cards (<span id="rateCountTab">0</span>)</div>
    <div class="tab" data-tab="rules" onclick="switchTab(this)">GoRules Decision Models (<span id="ruleCountTab">0</span>)</div>
  </div>

  <!-- Tab 1: Catalog & Assignment Overview -->
  <div id="tabCatalog">
    <div class="card">
      <div class="card-header">
        Synchronized Catalog &amp; Assigned Rates
        <span class="subtitle" id="catalogSubtitle">Showing all offerings</span>
      </div>
      <table>
        <thead>
          <tr>
            <th>Category</th>
            <th>Identifier / SKU</th>
            <th>Title / Specification</th>
            <th>Status</th>
            <th>Assigned Rate / Model</th>
            <th>Meter</th>
            <th class="num">Actions</th>
          </tr>
        </thead>
        <tbody id="catalogBody">
          <tr><td colspan="7" class="empty">Loading catalog and rates…</td></tr>
        </tbody>
      </table>
    </div>
  </div>

  <!-- Tab 2: Rate Cards Table -->
  <div id="tabRates" style="display:none">
    <div class="card">
      <div class="card-header">
        Configured Rate Cards
        <span class="subtitle">Direct rate matching index with 4-way fallback (Flat, Tiered JSON, and GoRule bindings)</span>
      </div>
      <table>
        <thead>
          <tr>
            <th>ID</th>
            <th>Scope / Tenant</th>
            <th>Resource Type</th>
            <th>Catalog Item</th>
            <th>Machine Type</th>
            <th>Meter Name</th>
            <th>Model</th>
            <th class="num">Price / Rule Details</th>
            <th class="num">Hourly Rate</th>
            <th>Effective</th>
            <th class="num">Actions</th>
          </tr>
        </thead>
        <tbody id="ratesBody">
          <tr><td colspan="11" class="empty">Loading rates…</td></tr>
        </tbody>
      </table>
    </div>
  </div>

  <!-- Tab 3: GoRules Decision Models -->
  <div id="tabRules" style="display:none">
    <div class="card">
      <div class="card-header">
        Configured GoRules Decision Models (Zen Engine)
        <button class="btn-sm primary" onclick="openCreateRuleModal()">+ Add Decision Model</button>
      </div>
      <table>
        <thead>
          <tr>
            <th>ID</th>
            <th>Model Name</th>
            <th>Version</th>
            <th>Created</th>
            <th>Last Updated</th>
            <th class="num">Actions</th>
          </tr>
        </thead>
        <tbody id="rulesBody">
          <tr><td colspan="6" class="empty">Loading decision models…</td></tr>
        </tbody>
      </table>
    </div>

    <!-- Production Rules & Authoring Guide -->
    <div class="card" style="margin-top: 1.5rem;">
      <div class="card-header" style="background:#f8fafc;">
        <div>
          <strong>Production Decision Models &amp; Authoring Guide</strong>
          <span style="font-weight:normal; color:var(--muted); font-size:0.82rem; margin-left:0.5rem;">
            Deep-dive into existing GoRules and how to author new ones
          </span>
        </div>
        <button class="btn-xs primary" onclick="openCreateRuleModal()">+ New Rule from Template</button>
      </div>

      <div style="padding: 1.25rem 1.5rem; line-height: 1.55; font-size: 0.88rem;">
        <!-- Two Production Rules Grid -->
        <div style="display:grid; grid-template-columns: 1fr 1fr; gap: 1.25rem; margin-bottom: 1.5rem;">
          
          <!-- Rule 1: compute-pricing.json -->
          <div style="border: 1px solid var(--border); border-radius: 8px; padding: 1rem 1.15rem; background:#fafbfc;">
            <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:0.5rem;">
              <span class="badge badge-gorule" style="font-size:0.75rem;">compute-pricing.json</span>
              <button class="btn-xs" onclick="viewRuleModal('compute-pricing.json')">View JSON Graph</button>
            </div>
            <h4 style="margin:0 0 0.35rem 0; font-size:0.95rem;">Rule 1: Instance Type &amp; Loyalty Discounts</h4>
            <p style="color:var(--muted); margin:0 0 0.75rem 0; font-size:0.83rem;">
              Maps machine shapes (e.g. <code>standard-4-16</code>) and tenant tiers (e.g. <code>gold</code>) to base $/hour rates and automatic percentage discounts.
            </p>
            <div style="font-size:0.82rem; margin-bottom:0.6rem;">
              <strong>DAG:</strong> <code>Request</code> &rarr; <code>Instance Type Rate (table)</code> &rarr; <code>Calculate Cost (expr)</code> &rarr; <code>Response</code>
            </div>
            <div style="font-size:0.82rem; background:white; border:1px solid var(--border); border-radius:6px; padding:0.6rem;">
              <div style="font-weight:600; margin-bottom:0.25rem; font-size:0.76rem; text-transform:uppercase; color:var(--muted);">Truth Table Matrix</div>
              <div>• <code>standard-2-8</code> + <code>gold</code> &rarr; $0.10/hr (20% off = <strong>$0.08/hr</strong>)</div>
              <div>• <code>standard-4-16</code> + <code>gold</code> &rarr; $0.20/hr (20% off = <strong>$0.16/hr</strong>)</div>
              <div>• <code>standard-8-32</code> + <code>gold</code> &rarr; $0.40/hr (20% off = <strong>$0.32/hr</strong>)</div>
              <div>• Standard tenants &rarr; 0% discount</div>
            </div>
            <div style="font-size:0.8rem; margin-top:0.6rem; color:var(--muted);">
              <strong>Formula:</strong> <code>(uptime_sec / 3600) * price_per_hour * (1 - discount_pct / 100)</code>
            </div>
          </div>

          <!-- Rule 2: committed-use-pricing.json -->
          <div style="border: 1px solid var(--border); border-radius: 8px; padding: 1rem 1.15rem; background:#fafbfc;">
            <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:0.5rem;">
              <span class="badge badge-gorule" style="font-size:0.75rem;">committed-use-pricing.json</span>
              <button class="btn-xs" onclick="viewRuleModal('committed-use-pricing.json')">View JSON Graph</button>
            </div>
            <h4 style="margin:0 0 0.35rem 0; font-size:0.95rem;">Rule 2: Committed-Use &amp; Sustained-Use</h4>
            <p style="color:var(--muted); margin:0 0 0.75rem 0; font-size:0.83rem;">
              Enterprise contract commitments with dynamic fallback to utilization-based sustained discounts and on-demand pricing.
            </p>
            <div style="font-size:0.82rem; margin-bottom:0.6rem;">
              <strong>DAG:</strong> <code>Request</code> &rarr; <code>CUD Agreement</code> &rarr; <code>Sustained Discount</code> &rarr; <code>Calculate Cost</code> &rarr; <code>Response</code>
            </div>
            <div style="font-size:0.82rem; background:white; border:1px solid var(--border); border-radius:6px; padding:0.6rem;">
              <div style="font-weight:600; margin-bottom:0.25rem; font-size:0.76rem; text-transform:uppercase; color:var(--muted);">Evaluation Ladder</div>
              <div>1. <strong>CUD:</strong> If <code>running_vms &le; committed_qty</code> &rarr; <strong>40-50% off</strong></div>
              <div>2. <strong>Sustained:</strong> If util &ge; 75% &rarr; <strong>20% off</strong>, &ge; 50% &rarr; <strong>10% off</strong></div>
              <div>3. <strong>On-Demand:</strong> Low utilization &rarr; <strong>0% off</strong></div>
            </div>
            <div style="font-size:0.8rem; margin-top:0.6rem; color:var(--muted);">
              <strong>Audit:</strong> Stamps cost entries with <code>CUD 1-year</code>, <code>Sustained-use</code>, or <code>On-demand</code>
            </div>
          </div>

        </div>

        <!-- Authoring Reference & Contract -->
        <div style="background:#f1f5f9; border-radius:8px; padding:1rem 1.25rem; font-size:0.84rem;">
          <h4 style="margin:0 0 0.5rem 0; font-size:0.9rem; color:var(--text);">How to Author &amp; Bind a New GoRule</h4>
          <div style="display:grid; grid-template-columns: 1fr 1fr; gap:1.25rem;">
            <div>
              <div style="font-weight:600; margin-bottom:0.25rem; color:var(--muted); font-size:0.76rem; text-transform:uppercase;">Input Parameters (Passed by Rating Engine)</div>
              <ul style="margin:0; padding-left:1.2rem; color:var(--text);">
                <li><code>instance_type</code>: Sizing spec (e.g. <code>"standard-2-8"</code>)</li>
                <li><code>catalog_item</code>: Catalog offering / SKU name</li>
                <li><code>tenant_tier</code>: Tenant tier label (e.g. <code>"gold"</code>)</li>
                <li><code>tenant_id</code>: Tenant ID / namespace</li>
                <li><code>value</code>: Metered quantity (e.g. seconds, tokens)</li>
              </ul>
            </div>
            <div>
              <div style="font-weight:600; margin-bottom:0.25rem; color:var(--muted); font-size:0.76rem; text-transform:uppercase;">Required Output Contract (Zen Output Node)</div>
              <ul style="margin:0; padding-left:1.2rem; color:var(--text);">
                <li><code>cost_amount</code> (number): Total dollar cost</li>
                <li><code>effective_rate</code> (number): Hourly or unit rate</li>
                <li><code>currency</code> (string): e.g. <code>'USD'</code></li>
                <li><code>description</code> (string): Description for invoices / reports</li>
              </ul>
            </div>
          </div>
          <div style="margin-top:0.75rem; padding-top:0.75rem; border-top:1px solid #cbd5e1; display:flex; justify-content:space-between; align-items:center;">
            <span>Documentation: Detailed specification in <code>docs/gorules-authoring-guide.md</code>.</span>
            <button class="btn-xs" onclick="openCreateRuleModal()">Open Rule Editor</button>
          </div>
        </div>

      </div>
    </div>
  </div>
</div>

<!-- Assign / Edit Rate Modal -->
<div class="modal-overlay" id="rateModal" style="display:none">
  <div class="modal">
    <h2 id="modalTitle">Assign Rate Card</h2>
    <p>Configure pricing for catalog sizing or template metering across the two-tier architecture: Flat, Tiered (JSON), or GoRule.</p>

    <div class="form-group">
      <label>Pricing Model</label>
      <div class="model-selector">
        <div class="model-card active" id="cardFlat" onclick="selectRateModel('flat')">
          <div class="title">Flat / Simple</div>
          <div class="sub">Linear price/unit</div>
        </div>
        <div class="model-card" id="cardTiered" onclick="selectRateModel('tiered')">
          <div class="title">Tiered (JSON)</div>
          <div class="sub">Graduated brackets</div>
        </div>
        <div class="model-card" id="cardGoRule" onclick="selectRateModel('gorule')">
          <div class="title">GoRule</div>
          <div class="sub">Decision graph</div>
        </div>
      </div>
    </div>

    <div class="form-row">
      <div class="form-group">
        <label>Resource Type</label>
        <select id="rateResourceType" onchange="onResourceTypeChange()">
          <option value="compute_instance">compute_instance (VM)</option>
          <option value="cluster">cluster (Kubernetes)</option>
          <option value="bare_metal">bare_metal (BM)</option>
          <option value="model">model (MaaS / AI)</option>
        </select>
      </div>
      <div class="form-group">
        <label>Machine Instance Type</label>
        <input type="text" id="rateInstanceType" placeholder="e.g. standard-4-16 (or blank for all)">
        <div class="hint">OSAC hardware shape, from billing_dimensions.instance_type.</div>
      </div>
    </div>

    <div class="form-row">
      <div class="form-group">
        <label>Catalog Item</label>
        <input type="text" id="rateCatalogItem" placeholder="e.g. sim-catalog-standard (or blank for all)">
        <div class="hint">OSAC catalog item name/SKU, from catalog_item_id.</div>
      </div>
      <div class="form-group">
        <label>Meter Name</label>
        <input type="text" id="rateMeterName" placeholder="e.g. vm_uptime_seconds">
        <div class="hint" id="meterHint">Default for VMs: vm_uptime_seconds</div>
      </div>
    </div>

    <div class="form-row">
      <div class="form-group">
        <label>Cost Layer</label>
        <select id="rateCostType">
          <option value="Infrastructure">Infrastructure</option>
          <option value="Supplementary">Supplementary</option>
        </select>
      </div>
      <div class="form-group">
        <label>Tenant ID (Optional)</label>
        <input type="text" id="rateTenantId" placeholder="Blank for global default">
      </div>
    </div>

    <!-- Section 1: Flat Pricing -->
    <div id="sectionFlat">
      <div class="form-row">
        <div class="form-group">
          <label>Price Mode</label>
          <select id="ratePriceMode" onchange="onPriceModeChange()">
            <option value="hourly">Hourly Rate ($ / hour)</option>
            <option value="unit">Direct Unit Price ($ / unit)</option>
          </select>
        </div>
        <div class="form-group">
          <label id="priceInputLabel">Price ($ / hour)</label>
          <input type="number" step="any" min="0" id="ratePriceInput" placeholder="0.10" oninput="updateCalculatedPrice()">
        </div>
      </div>
      <div class="form-group">
        <label>Calculated Stored Price per Unit (price_per_unit)</label>
        <input type="text" id="ratePricePerUnit" readonly style="background:#f9fafb;font-family:monospace;font-weight:600">
        <div class="hint" id="calcHint">Stored as $ / second for uptime meters (hourly / 3600).</div>
      </div>
    </div>

    <!-- Section 2: Tiered Pricing (JSON) -->
    <div id="sectionTiered" style="display:none">
      <div class="form-row">
        <div class="form-group">
          <label>Tier Mode</label>
          <select id="rateTierMode" onchange="onTierModeChange()">
            <option value="cumulative">Cumulative (windowed consumption)</option>
            <option value="per_event">Per Event (independent per call)</option>
          </select>
        </div>
        <div class="form-group" id="tierPeriodGroup">
          <label>Accumulation Window (Period)</label>
          <select id="rateTierPeriod">
            <option value="monthly">monthly (calendar month)</option>
            <option value="weekly">weekly (calendar week)</option>
            <option value="daily">daily (calendar day)</option>
            <option value="5h">5h (5-hour window)</option>
            <option value="7d">7d (7-day window)</option>
          </select>
        </div>
      </div>
      <div class="form-group">
        <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:0.3rem">
          <label style="margin:0">Tiers JSON Definition</label>
          <div style="display:flex;gap:0.3rem">
            <button class="btn-xs" type="button" onclick="loadTierTemplate('standard')">3-Tier Template</button>
            <button class="btn-xs" type="button" onclick="loadTierTemplate('free')">Free Allowance</button>
          </div>
        </div>
        <textarea id="rateTiersJSON" rows="5" style="font-family:monospace;font-size:0.8rem" oninput="validateTiersJSON()"></textarea>
        <div class="hint">Array of graduated brackets: [{"up_to": 20, "price_per_unit": 0}, {"up_to": 120, "price_per_unit": 0.08}, {"up_to": null, "price_per_unit": 0.07}]</div>
        <div id="tiersPreview" class="preview-box">Valid tier format required</div>
      </div>
    </div>

    <!-- Section 3: GoRule Binding -->
    <div id="sectionGoRule" style="display:none">
      <div class="form-group">
        <label>Assigned GoRule Decision Model</label>
        <select id="rateRuleSelect" onchange="onRuleSelectChange()"></select>
        <div class="hint">Select an existing decision model from pricing_rules or enter a new one.</div>
      </div>
      <div class="form-group" id="customRuleGroup" style="display:none">
        <label>Custom Rule Name</label>
        <input type="text" id="rateCustomRuleName" placeholder="e.g. custom-pricing.json">
      </div>
      <div class="preview-box" id="ruleInfoBox" style="margin-top:0.5rem">
        This rate card will bind to the selected GoRules decision model. The Zen engine will interpret the rule dynamically during rating sweeps.
      </div>
    </div>

    <div class="form-row" style="margin-top:0.75rem">
      <div class="form-group">
        <label>Currency</label>
        <input type="text" id="rateCurrency" value="USD">
      </div>
      <div class="form-group">
        <label>Description</label>
        <input type="text" id="rateDescription" placeholder="e.g. Standard VM rate card">
      </div>
    </div>

    <div class="modal-btns">
      <button class="btn-secondary" onclick="closeAssignModal()">Cancel</button>
      <button class="btn-primary" onclick="submitRateForm()">Save Rate Card</button>
    </div>
  </div>
</div>

<!-- GoRule Create/Edit Modal -->
<div class="modal-overlay" id="ruleModal" style="display:none">
  <div class="modal wide">
    <h2 id="ruleModalTitle">GoRule Decision Model</h2>
    <p>Upload or update a JSON Decision Model (JDM) graph. Saved rules hot-reload in the Zen engine with zero downtime.</p>

    <div class="form-group">
      <label>Model Name</label>
      <input type="text" id="ruleNameInput" placeholder="e.g. compute-pricing.json">
      <div class="hint">Unique name referencing the decision graph.</div>
    </div>

    <div class="form-group">
      <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:0.3rem">
        <label style="margin:0">Decision Graph JSON (JDM)</label>
        <div style="display:flex;gap:0.3rem">
          <button class="btn-xs" type="button" onclick="formatRuleJSON()">Format JSON</button>
          <button class="btn-xs" type="button" onclick="loadRuleTemplate()">Load Template</button>
        </div>
      </div>
      <textarea id="ruleJSONInput" rows="14" style="font-family:monospace;font-size:0.8rem"></textarea>
      <div id="ruleJSONError" class="hint" style="color:var(--red)"></div>
    </div>

    <div class="modal-btns">
      <button class="btn-secondary" onclick="closeRuleModal()">Cancel</button>
      <button class="btn-primary" onclick="submitRuleForm()">Save Decision Model</button>
    </div>
  </div>
</div>

<!-- General JSON View Modal -->
<div class="modal-overlay" id="jsonModal" style="display:none">
  <div class="modal wide">
    <h2 id="jsonModalTitle">JSON Document</h2>
    <div class="form-group" style="margin-top:0.5rem">
      <textarea id="jsonModalContent" rows="18" readonly style="font-family:monospace;font-size:0.8rem;background:#f9fafb"></textarea>
    </div>
    <div class="modal-btns">
      <button class="btn-secondary" onclick="copyJSONModal()">Copy JSON</button>
      <button class="btn-secondary" onclick="closeJSONModal()">Close</button>
    </div>
  </div>
</div>

<!-- Token Modal -->
<div class="modal-overlay" id="tokenModal" style="display:none">
  <div class="modal">
    <h2>API Authentication Token</h2>
    <p>Paste an OSAC service token or JWT Bearer token to authorize operations. Stored locally in your browser.</p>
    <div class="form-group">
      <textarea id="tokenInput" placeholder="Bearer eyJhbGciOi..." style="height:100px;font-family:monospace;font-size:0.8rem"></textarea>
    </div>
    <div class="modal-btns">
      <button class="btn-secondary" onclick="clearToken()">Clear</button>
      <button class="btn-secondary" onclick="hideTokenModal()">Close</button>
      <button class="btn-primary" onclick="saveToken()">Save Token</button>
    </div>
  </div>
</div>

<script>
const TOKEN_KEY = 'koku_report_token';
const $ = id => document.getElementById(id);

let state = {
  catalogItems: [],
  instanceTypes: [],
  rates: [],
  rules: [],
  activeTab: 'catalog',
  currentModel: 'flat'
};

function getToken() { return localStorage.getItem(TOKEN_KEY) || ''; }
function updateTokenBtn() {
  const tok = getToken();
  const btn = $('tokenBtn');
  if (tok) { btn.textContent = 'Token Set'; btn.classList.add('has-token'); }
  else { btn.textContent = 'Token'; btn.classList.remove('has-token'); }
}
function showTokenModal() { $('tokenInput').value = getToken(); $('tokenModal').style.display = 'flex'; }
function hideTokenModal() { $('tokenModal').style.display = 'none'; }
function saveToken() {
  localStorage.setItem(TOKEN_KEY, $('tokenInput').value.trim());
  updateTokenBtn(); hideTokenModal(); loadAll();
}
function clearToken() { localStorage.removeItem(TOKEN_KEY); updateTokenBtn(); hideTokenModal(); }

async function api(path, options = {}) {
  const headers = Object.assign({}, options.headers || {});
  const tok = getToken();
  if (tok) headers['Authorization'] = 'Bearer ' + tok;
  if (options.body && typeof options.body === 'string' && !headers['Content-Type']) {
    headers['Content-Type'] = 'application/json';
  }
  const res = await fetch(window.location.origin + path, Object.assign({}, options, { headers }));
  if (res.status === 401) {
    showTokenModal();
    throw new Error('401 Unauthorized — please set an authentication token');
  }
  return res;
}

function showBanner(type, msg) {
  const b = $(type + 'Banner');
  b.textContent = msg;
  b.style.display = 'block';
  setTimeout(() => { b.style.display = 'none'; }, 6000);
}

function switchTab(el) {
  document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
  el.classList.add('active');
  state.activeTab = el.dataset.tab;
  $('tabCatalog').style.display = state.activeTab === 'catalog' ? 'block' : 'none';
  $('tabRates').style.display = state.activeTab === 'rates' ? 'block' : 'none';
  $('tabRules').style.display = state.activeTab === 'rules' ? 'block' : 'none';
  renderAll();
}

async function loadAll() {
  $('errorBanner').style.display = 'none';
  try {
    const [catRes, ratesRes, rulesRes] = await Promise.all([
      api('/api/v1/catalog'),
      api('/api/v1/rates'),
      api('/api/v1/rules').catch(() => ({ ok: false }))
    ]);

    if (!catRes.ok) throw new Error('Failed to load catalog: ' + catRes.status);
    if (!ratesRes.ok) throw new Error('Failed to load rates: ' + ratesRes.status);

    const catData = await catRes.json();
    const ratesData = await ratesRes.json();

    state.catalogItems = catData.catalog_items || [];
    state.instanceTypes = catData.instance_types || [];
    state.rates = ratesData.rates || [];

    if (rulesRes && rulesRes.ok) {
      const rulesData = await rulesRes.json();
      state.rules = rulesData.rules || [];
    } else {
      state.rules = [];
    }

    $('lastUpdated').textContent = 'Synced ' + new Date().toLocaleTimeString();
    renderAll();
  } catch (err) {
    showBanner('error', err.message);
  }
}

function findMatchingRate(resourceType, instanceType) {
  const exact = state.rates.find(r => (!r.effective_to || new Date(r.effective_to) > new Date()) &&
    r.resource_type.toLowerCase() === resourceType.toLowerCase() &&
    (!r.catalog_item || r.catalog_item === '') &&
    r.instance_type === instanceType);
  if (exact) return exact;

  return state.rates.find(r => (!r.effective_to || new Date(r.effective_to) > new Date()) &&
    r.resource_type.toLowerCase() === resourceType.toLowerCase() &&
    (!r.catalog_item || r.catalog_item === '') &&
    (!r.instance_type || r.instance_type === ''));
}

function findMatchingCatalogRate(resourceType, catalogItem) {
  const exact = state.rates.find(r => (!r.effective_to || new Date(r.effective_to) > new Date()) &&
    r.resource_type.toLowerCase() === resourceType.toLowerCase() &&
    r.catalog_item === catalogItem);
  if (exact) return exact;
  return state.rates.find(r => (!r.effective_to || new Date(r.effective_to) > new Date()) &&
    r.resource_type.toLowerCase() === resourceType.toLowerCase() &&
    (!r.catalog_item || r.catalog_item === '') &&
    (!r.instance_type || r.instance_type === ''));
}

function checkGoRuleCoverage(sku) {
  // Check if sku is referenced in compute-pricing.json or other active rules
  for (const rule of state.rules) {
    if (rule.rule_json) {
      const str = typeof rule.rule_json === 'string' ? rule.rule_json : JSON.stringify(rule.rule_json);
      if (str.includes(sku)) {
        return rule.name;
      }
    }
  }
  return null;
}

function escapeHTML(str) {
  if (str === null || str === undefined) return '';
  return String(str)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;');
}

function formatPriceDisplay(rate, fallbackRule) {
  if (rate && rate.rule_name) {
    return '<span class="badge badge-gorule">GoRule: ' + escapeHTML(rate.rule_name) + '</span> ' +
           '<button class="btn-xs" onclick="viewRuleModal(\'' + escapeHTML(rate.rule_name) + '\')">View Rule</button>';
  }
  if (rate && rate.tiers && rate.tiers.length > 0) {
    const mode = rate.tier_mode || 'cumulative';
    const period = rate.tier_period ? ': ' + rate.tier_period : '';
    return '<span class="badge badge-supp">Tiered (' + rate.tiers.length + ' bands, ' + escapeHTML(mode + period) + ')</span> ' +
           '<button class="btn-xs" onclick="viewTiersModal(' + rate.id + ')">View Tiers</button>';
  }
  if (rate) {
    const price = parseFloat(rate.price_per_unit) || 0;
    if (rate.meter_name && rate.meter_name.includes('seconds')) {
      const hourly = price * 3600;
      return '<span class="badge badge-priced">$' + hourly.toFixed(4) + ' / hr</span>' +
             ' <span style="font-size:0.75rem;color:var(--muted)">($' + price.toFixed(8) + '/s)</span>';
    }
    return '<span class="badge badge-priced">$' + price.toFixed(6) + ' / ' + escapeHTML(rate.currency || 'USD') + '</span>';
  }
  if (fallbackRule) {
    return '<span class="badge badge-gorule">GoRule: ' + escapeHTML(fallbackRule) + '</span> ' +
           '<button class="btn-xs" onclick="viewRuleModal(\'' + escapeHTML(fallbackRule) + '\')">View Rule</button>';
  }
  return '<span class="badge badge-unpriced">Unpriced</span>';
}

function renderAll() {
  const query = $('filterSearch').value.toLowerCase().trim();
  const categoryFilter = $('filterType').value;
  const pricingFilter = $('filterPricing').value;

  const now = new Date();
  const activeRates = state.rates.filter(r => !r.effective_to || new Date(r.effective_to) > now);

  $('kInstances').textContent = state.instanceTypes.length;
  $('kCatalog').textContent = state.catalogItems.length;
  $('kRates').textContent = activeRates.length;
  $('kRules').textContent = state.rules.length;
  $('rateCountTab').textContent = activeRates.length;
  $('ruleCountTab').textContent = state.rules.length;

  // Build unified catalog entries
  const unified = [];

  for (const it of state.instanceTypes) {
    const rate = findMatchingRate('compute_instance', it.name);
    const coveredByRule = !rate ? checkGoRuleCoverage(it.name) : null;
    unified.push({
      category: 'instance_type',
      categoryLabel: 'Machine Type',
      resourceType: 'compute_instance',
      sku: it.name,
      id: it.instance_type_id,
      title: it.name,
      specs: (it.cores || 0) + ' vCPU, ' + (it.memory_gib || 0) + ' GiB RAM',
      status: it.state || 'active',
      rate: rate,
      fallbackRule: coveredByRule
    });
  }

  for (const ci of state.catalogItems) {
    let resType = 'compute_instance';
    let catLabel = 'Compute Offering';
    if (ci.item_type && ci.item_type.includes('cluster')) { resType = 'cluster'; catLabel = 'Cluster Offering'; }
    else if (ci.item_type && ci.item_type.includes('bare_metal')) { resType = 'bare_metal'; catLabel = 'Bare Metal Offering'; }
    else if (ci.item_type && ci.item_type.includes('model')) { resType = 'model'; catLabel = 'Model Offering'; }

    const rate = findMatchingCatalogRate(resType, ci.name) || findMatchingCatalogRate(resType, ci.catalog_item_id);
    const coveredByRule = !rate ? checkGoRuleCoverage(ci.name) : null;
    unified.push({
      category: resType,
      categoryLabel: catLabel,
      resourceType: resType,
      sku: ci.name,
      id: ci.catalog_item_id,
      title: ci.title || ci.name,
      specs: ci.description || ('Template: ' + (ci.template || '—')),
      status: ci.published ? 'published' : 'draft',
      rate: rate,
      fallbackRule: coveredByRule
    });
  }

  // Filter unified catalog
  const filtered = unified.filter(item => {
    if (categoryFilter && item.category !== categoryFilter) return false;
    const isPriced = !!item.rate || !!item.fallbackRule;
    if (pricingFilter === 'priced' && !isPriced) return false;
    if (pricingFilter === 'unpriced' && isPriced) return false;
    if (query) {
      const haystack = (item.sku + ' ' + item.title + ' ' + item.specs + ' ' + (item.rate ? item.rate.meter_name : '')).toLowerCase();
      if (!haystack.includes(query)) return false;
    }
    return true;
  });

  // Render Catalog Table
  const catBody = $('catalogBody');
  if (filtered.length === 0) {
    catBody.innerHTML = '<tr><td colspan="7" class="empty">No catalog offerings match the selected filters.</td></tr>';
  } else {
    catBody.innerHTML = filtered.map(item => {
      let badgeClass = 'badge-inst';
      if (item.category === 'cluster') badgeClass = 'badge-cat';
      else if (item.category === 'bare_metal') badgeClass = 'badge-bm';
      else if (item.category === 'model') badgeClass = 'badge-supp';

      const meterText = item.rate ? '<span class="code-text">' + escapeHTML(item.rate.meter_name) + '</span>' : '<span style="color:var(--muted)">—</span>';
      const actionText = item.rate ? 'Edit Rate' : 'Assign Rate';

      return '<tr>' +
        '<td><span class="badge ' + escapeHTML(badgeClass) + '">' + escapeHTML(item.categoryLabel) + '</span></td>' +
        '<td><span class="code-text" style="font-weight:600">' + escapeHTML(item.sku) + '</span></td>' +
        '<td><div><strong>' + escapeHTML(item.title) + '</strong></div><div style="font-size:0.78rem;color:var(--muted)">' + escapeHTML(item.specs) + '</div></td>' +
        '<td><span style="font-size:0.8rem;text-transform:capitalize">' + escapeHTML(item.status) + '</span></td>' +
        '<td>' + formatPriceDisplay(item.rate, item.fallbackRule) + '</td>' +
        '<td>' + meterText + '</td>' +
        '<td class="num"><button class="btn-sm primary btn-assign" data-resource-type="' + escapeHTML(item.resourceType) + '" data-category="' + escapeHTML(item.category) + '" data-sku="' + escapeHTML(item.sku) + '">' + escapeHTML(actionText) + '</button></td>' +
      '</tr>';
    }).join('');
  }

  // Render Rates Table
  const ratesBody = $('ratesBody');
  if (activeRates.length === 0) {
    ratesBody.innerHTML = '<tr><td colspan="11" class="empty">No active rates configured.</td></tr>';
  } else {
    ratesBody.innerHTML = activeRates.map(r => {
      const tenant = r.tenant_id ? '<span class="code-text">' + escapeHTML(r.tenant_id) + '</span>' : '<span class="badge badge-infra">Global</span>';
      const effFrom = r.effective_from ? new Date(r.effective_from).toLocaleDateString() : '—';

      let modelBadge = '<span class="badge badge-infra">Flat</span>';
      let details = '$' + (parseFloat(r.price_per_unit) || 0).toFixed(8);
      let hourly = '—';

      if (r.rule_name) {
        modelBadge = '<span class="badge badge-gorule">GoRule</span>';
        details = '<span class="code-text">' + escapeHTML(r.rule_name) + '</span> ' +
                  '<button class="btn-xs" onclick="viewRuleModal(\'' + escapeHTML(r.rule_name) + '\')">JSON</button>';
      } else if (r.tiers && r.tiers.length > 0) {
        modelBadge = '<span class="badge badge-supp">Tiered (' + escapeHTML(r.tier_mode || 'cumulative') + ')</span>';
        details = r.tiers.length + ' brackets ' +
                  '<button class="btn-xs" onclick="viewTiersModal(' + r.id + ')">JSON</button>';
      } else {
        const price = parseFloat(r.price_per_unit) || 0;
        if (r.meter_name && r.meter_name.includes('seconds')) {
          hourly = '$' + (price * 3600).toFixed(4) + '/hr';
        }
      }

      return '<tr>' +
        '<td>' + escapeHTML(r.id) + '</td>' +
        '<td>' + tenant + '</td>' +
        '<td><span class="code-text">' + escapeHTML(r.resource_type) + '</span></td>' +
        '<td><span class="code-text">' + escapeHTML(r.catalog_item || '*(any)*') + '</span></td>' +
        '<td><span class="code-text">' + escapeHTML(r.instance_type || '*(any)*') + '</span></td>' +
        '<td><span class="code-text">' + escapeHTML(r.meter_name) + '</span></td>' +
        '<td>' + modelBadge + '</td>' +
        '<td class="num">' + details + '</td>' +
        '<td class="num">' + hourly + '</td>' +
        '<td>' + effFrom + '</td>' +
        '<td class="num"><button class="btn-sm btn-retire" data-rate-id="' + escapeHTML(r.id) + '" style="color:var(--red);border-color:#fca5a5">Retire</button></td>' +
      '</tr>';
    }).join('');
  }

  // Render Rules Table
  const rulesBody = $('rulesBody');
  if (state.rules.length === 0) {
    rulesBody.innerHTML = '<tr><td colspan="6" class="empty">No GoRules decision models configured.</td></tr>';
  } else {
    rulesBody.innerHTML = state.rules.map(rl => {
      const updated = rl.updated_at ? new Date(rl.updated_at).toLocaleString() : '—';
      const created = rl.created_at ? new Date(rl.created_at).toLocaleDateString() : '—';
      return '<tr>' +
        '<td>' + escapeHTML(rl.id) + '</td>' +
        '<td><strong class="code-text">' + escapeHTML(rl.name) + '</strong></td>' +
        '<td><span class="badge badge-priced">v' + escapeHTML(rl.version) + '</span></td>' +
        '<td>' + created + '</td>' +
        '<td>' + updated + '</td>' +
        '<td class="num">' +
          '<button class="btn-xs" onclick="viewRuleModal(\'' + escapeHTML(rl.name) + '\')" style="margin-right:0.3rem">View JSON</button>' +
          '<button class="btn-xs" onclick="openEditRuleModal(\'' + escapeHTML(rl.name) + '\')" style="margin-right:0.3rem">Edit</button>' +
          '<button class="btn-xs" onclick="deleteRule(\'' + escapeHTML(rl.name) + '\')" style="color:var(--red);border-color:#fca5a5">Delete</button>' +
        '</td>' +
      '</tr>';
    }).join('');
  }
}

function selectRateModel(model) {
  state.currentModel = model;
  $('cardFlat').classList.toggle('active', model === 'flat');
  $('cardTiered').classList.toggle('active', model === 'tiered');
  $('cardGoRule').classList.toggle('active', model === 'gorule');

  $('sectionFlat').style.display = model === 'flat' ? 'block' : 'none';
  $('sectionTiered').style.display = model === 'tiered' ? 'block' : 'none';
  $('sectionGoRule').style.display = model === 'gorule' ? 'block' : 'none';

  if (model === 'tiered') validateTiersJSON();
  if (model === 'gorule') populateRulesDropdown();
}

function populateRulesDropdown() {
  const sel = $('rateRuleSelect');
  sel.innerHTML = '';
  for (const r of state.rules) {
    const opt = document.createElement('option');
    opt.value = r.name;
    opt.textContent = r.name + ' (v' + r.version + ')';
    sel.appendChild(opt);
  }
  const customOpt = document.createElement('option');
  customOpt.value = '__custom__';
  customOpt.textContent = '+ Specify new rule name…';
  sel.appendChild(customOpt);
  onRuleSelectChange();
}

function onRuleSelectChange() {
  const isCustom = $('rateRuleSelect').value === '__custom__';
  $('customRuleGroup').style.display = isCustom ? 'block' : 'none';
}

function onResourceTypeChange() {
  const rt = $('rateResourceType').value;
  const meterInput = $('rateMeterName');
  const hint = $('meterHint');
  if (rt === 'compute_instance') {
    meterInput.value = 'vm_uptime_seconds';
    hint.textContent = 'Default: vm_uptime_seconds (uptime) or vm_cpu_core_seconds';
  } else if (rt === 'cluster') {
    meterInput.value = 'cluster_uptime_seconds';
    hint.textContent = 'Default: cluster_uptime_seconds (control plane) or cluster_worker_node_seconds';
  } else if (rt === 'bare_metal') {
    meterInput.value = 'bm_uptime_seconds';
    hint.textContent = 'Default: bm_uptime_seconds';
  } else if (rt === 'model') {
    meterInput.value = 'maas_tokens_in';
    hint.textContent = 'Default: maas_tokens_in, maas_tokens_out, or maas_requests';
  }
  updateCalculatedPrice();
}

function onPriceModeChange() {
  const mode = $('ratePriceMode').value;
  $('priceInputLabel').textContent = mode === 'hourly' ? 'Price ($ / hour)' : 'Price ($ / unit)';
  updateCalculatedPrice();
}

function updateCalculatedPrice() {
  const mode = $('ratePriceMode').value;
  const raw = parseFloat($('ratePriceInput').value) || 0;
  let pricePerUnit = raw;
  if (mode === 'hourly') {
    pricePerUnit = raw / 3600;
    $('calcHint').textContent = 'Stored as ' + pricePerUnit.toFixed(10) + ' USD / second (hourly / 3600).';
  } else {
    $('calcHint').textContent = 'Stored as direct price per metered unit.';
  }
  $('ratePricePerUnit').value = pricePerUnit.toFixed(10);
}

function onTierModeChange() {
  const mode = $('rateTierMode').value;
  $('tierPeriodGroup').style.display = mode === 'cumulative' ? 'block' : 'none';
}

function loadTierTemplate(type) {
  if (type === 'free') {
    $('rateTiersJSON').value = JSON.stringify([
      { up_to: 20, price_per_unit: 0 },
      { up_to: null, price_per_unit: 0.08 }
    ], null, 2);
  } else {
    $('rateTiersJSON').value = JSON.stringify([
      { up_to: 20, price_per_unit: 0 },
      { up_to: 120, price_per_unit: 0.08 },
      { up_to: null, price_per_unit: 0.07 }
    ], null, 2);
  }
  validateTiersJSON();
}

function validateTiersJSON() {
  const val = $('rateTiersJSON').value.trim();
  const preview = $('tiersPreview');
  if (!val) {
    preview.textContent = 'Paste or enter JSON array of tier brackets.';
    preview.style.color = 'var(--muted)';
    return null;
  }
  try {
    const parsed = JSON.parse(val);
    if (!Array.isArray(parsed) || parsed.length === 0) {
      preview.textContent = 'Error: Must be a non-empty array of tier objects.';
      preview.style.color = 'var(--red)';
      return null;
    }
    const summary = parsed.map((t, idx) => {
      const up = t.up_to === null || t.up_to === undefined ? 'unlimited' : t.up_to;
      return 'Tier ' + (idx + 1) + ': up to ' + up + ' @ $' + t.price_per_unit;
    }).join('; ');
    preview.textContent = 'Parsed ' + parsed.length + ' brackets: ' + summary;
    preview.style.color = 'var(--green)';
    return parsed;
  } catch (err) {
    preview.textContent = 'Syntax Error: ' + err.message;
    preview.style.color = 'var(--red)';
    return null;
  }
}

function openAssignModal() {
  $('modalTitle').textContent = 'Assign Rate Card';
  $('rateResourceType').value = 'compute_instance';
  $('rateInstanceType').value = '';
  $('rateCatalogItem').value = '';
  $('rateMeterName').value = 'vm_uptime_seconds';
  $('rateCostType').value = 'Infrastructure';
  $('ratePriceMode').value = 'hourly';
  $('ratePriceInput').value = '0.10';
  $('rateTenantId').value = '';
  $('rateDescription').value = '';
  selectRateModel('flat');
  loadTierTemplate('standard');
  onResourceTypeChange();
  $('rateModal').style.display = 'flex';
}

function openAssignModalFor(resourceType, sku, category) {
  openAssignModal();
  $('modalTitle').textContent = 'Assign Rate: ' + sku;
  $('rateResourceType').value = resourceType;
  if (category === 'instance_type') {
    $('rateInstanceType').value = sku;
  } else {
    $('rateCatalogItem').value = sku;
  }
  onResourceTypeChange();
  $('rateDescription').value = 'Rate for ' + sku;
}

function closeAssignModal() { $('rateModal').style.display = 'none'; }

async function submitRateForm() {
  const resourceType = $('rateResourceType').value.trim();
  const instanceType = $('rateInstanceType').value.trim();
  const catalogItem = $('rateCatalogItem').value.trim();
  const meterName = $('rateMeterName').value.trim();
  const costType = $('rateCostType').value;
  const currency = $('rateCurrency').value.trim() || 'USD';
  const description = $('rateDescription').value.trim();
  const tenantId = $('rateTenantId').value.trim();

  if (!resourceType || !meterName) {
    alert('Resource Type and Meter Name are required.');
    return;
  }

  const payload = {
    resource_type: resourceType,
    instance_type: instanceType,
    catalog_item: catalogItem,
    meter_name: meterName,
    cost_type: costType,
    currency: currency,
    description: description
  };
  if (tenantId) payload.tenant_id = tenantId;

  if (state.currentModel === 'flat') {
    const pricePerUnit = $('ratePricePerUnit').value.trim();
    if (!pricePerUnit || parseFloat(pricePerUnit) < 0) {
      alert('Valid price is required.');
      return;
    }
    payload.price_per_unit = pricePerUnit;
    payload.tier_mode = 'per_event';
  } else if (state.currentModel === 'tiered') {
    const tiers = validateTiersJSON();
    if (!tiers) {
      alert('Valid Tiers JSON is required.');
      return;
    }
    payload.price_per_unit = '0';
    payload.tier_mode = $('rateTierMode').value;
    payload.tier_period = payload.tier_mode === 'cumulative' ? $('rateTierPeriod').value : '';
    payload.tiers = tiers.map(t => ({
      up_to: t.up_to === null || t.up_to === undefined || t.up_to === '' ? null : Number(t.up_to),
      price_per_unit: String(t.price_per_unit)
    }));
  } else if (state.currentModel === 'gorule') {
    let ruleName = $('rateRuleSelect').value;
    if (ruleName === '__custom__') {
      ruleName = $('rateCustomRuleName').value.trim();
    }
    if (!ruleName) {
      alert('GoRule model name is required.');
      return;
    }
    payload.price_per_unit = '0';
    payload.rule_name = ruleName;
    payload.tier_mode = 'per_event';
  }

  try {
    const res = await api('/api/v1/rates', {
      method: 'POST',
      body: JSON.stringify(payload)
    });
    if (!res.ok) {
      const errData = await res.json().catch(() => ({}));
      throw new Error(errData.error || ('HTTP ' + res.status));
    }
    closeAssignModal();
    showBanner('success', 'Rate card successfully created/updated.');
    loadAll();
  } catch (err) {
    alert('Failed to save rate: ' + err.message);
  }
}

async function retireRate(id) {
  if (!confirm('Retire rate #' + id + '? It will no longer apply to future metering sweeps.')) return;
  try {
    const res = await api('/api/v1/rates/' + id, { method: 'DELETE' });
    if (!res.ok) throw new Error('HTTP ' + res.status);
    showBanner('success', 'Rate #' + id + ' retired successfully.');
    loadAll();
  } catch (err) {
    alert('Failed to retire rate: ' + err.message);
  }
}

function openCreateRuleModal() {
  $('ruleModalTitle').textContent = 'Create Decision Model';
  $('ruleNameInput').value = '';
  $('ruleNameInput').readOnly = false;
  $('ruleJSONInput').value = '';
  loadRuleTemplate();
  $('ruleJSONError').textContent = '';
  $('ruleModal').style.display = 'flex';
}

function openEditRuleModal(name) {
  const rule = state.rules.find(r => r.name === name);
  if (!rule) return;
  $('ruleModalTitle').textContent = 'Edit Decision Model: ' + name;
  $('ruleNameInput').value = rule.name;
  $('ruleNameInput').readOnly = true;
  const jsonStr = typeof rule.rule_json === 'string' ? rule.rule_json : JSON.stringify(rule.rule_json, null, 2);
  $('ruleJSONInput').value = jsonStr;
  $('ruleJSONError').textContent = '';
  $('ruleModal').style.display = 'flex';
}

function closeRuleModal() { $('ruleModal').style.display = 'none'; }

function loadRuleTemplate() {
  const tpl = {
    nodes: [
      { id: "input-1", type: "inputNode", name: "Request" },
      {
        id: "rate-table",
        type: "decisionTableNode",
        name: "Pricing Table",
        content: {
          hitPolicy: "first",
          inputs: [
            { field: "instance_type", id: "c1", name: "Instance Type", type: "expression" },
            { field: "tenant_tier", id: "c2", name: "Tenant Tier", type: "expression" }
          ],
          outputs: [
            { field: "price_per_hour", id: "o1", name: "Price/Hour ($)", type: "expression" },
            { field: "discount_pct", id: "o2", name: "Discount %", type: "expression" },
            { field: "description", id: "o3", name: "Description", type: "expression" }
          ],
          rules: [
            { _id: "r1", c1: "\"standard-4-16\"", c2: "\"gold\"", o1: "0.20", o2: "20", o3: "'4 cores Gold (20% off)'" },
            { _id: "r2", c1: "\"standard-4-16\"", c2: "", o1: "0.20", o2: "0", o3: "'4 cores standard'" },
            { _id: "r-def", c1: "", c2: "", o1: "0.10", o2: "0", o3: "'default rate'" }
          ]
        }
      },
      {
        id: "calc-cost",
        type: "expressionNode",
        name: "Calculate Cost",
        content: {
          expressions: [
            { id: "e1", key: "cost_amount", value: "(value / 3600) * price_per_hour * (1 - discount_pct / 100)" },
            { id: "e2", key: "effective_rate", value: "price_per_hour * (1 - discount_pct / 100)" },
            { id: "e3", key: "currency", value: "'USD'" }
          ]
        }
      }
    ],
    edges: [
      { id: "e-in-table", sourceId: "input-1", targetId: "rate-table" },
      { id: "e-table-calc", sourceId: "rate-table", targetId: "calc-cost" }
    ]
  };
  $('ruleJSONInput').value = JSON.stringify(tpl, null, 2);
}

function formatRuleJSON() {
  try {
    const parsed = JSON.parse($('ruleJSONInput').value);
    $('ruleJSONInput').value = JSON.stringify(parsed, null, 2);
    $('ruleJSONError').textContent = '';
  } catch (err) {
    $('ruleJSONError').textContent = 'Invalid JSON: ' + err.message;
  }
}

async function submitRuleForm() {
  const name = $('ruleNameInput').value.trim();
  const rawJSON = $('ruleJSONInput').value.trim();
  if (!name) { alert('Rule name is required'); return; }
  let parsed;
  try {
    parsed = JSON.parse(rawJSON);
  } catch (err) {
    $('ruleJSONError').textContent = 'Invalid JSON: ' + err.message;
    return;
  }

  try {
    const res = await api('/api/v1/rules', {
      method: 'POST',
      body: JSON.stringify({ name: name, rule_json: parsed })
    });
    if (!res.ok) {
      const err = await res.json().catch(() => ({}));
      throw new Error(err.error || ('HTTP ' + res.status));
    }
    closeRuleModal();
    showBanner('success', 'GoRule decision model "' + name + '" successfully saved.');
    loadAll();
  } catch (err) {
    alert('Failed to save rule: ' + err.message);
  }
}

async function deleteRule(name) {
  if (!confirm('Delete GoRule decision model "' + name + '"?')) return;
  try {
    const res = await api('/api/v1/rules/' + encodeURIComponent(name), { method: 'DELETE' });
    if (!res.ok) throw new Error('HTTP ' + res.status);
    showBanner('success', 'GoRule "' + name + '" deleted successfully.');
    loadAll();
  } catch (err) {
    alert('Failed to delete rule: ' + err.message);
  }
}

function viewRuleModal(name) {
  const rule = state.rules.find(r => r.name === name);
  $('jsonModalTitle').textContent = 'GoRule: ' + name;
  if (rule) {
    const str = typeof rule.rule_json === 'string' ? rule.rule_json : JSON.stringify(rule.rule_json, null, 2);
    $('jsonModalContent').value = str;
  } else {
    $('jsonModalContent').value = '// Rule definition loaded from disk or active in engine';
  }
  $('jsonModal').style.display = 'flex';
}

function viewTiersModal(rateId) {
  const rate = state.rates.find(r => r.id === rateId);
  $('jsonModalTitle').textContent = 'Graduated Tiers (Rate #' + rateId + ')';
  if (rate && rate.tiers) {
    $('jsonModalContent').value = JSON.stringify(rate.tiers, null, 2);
  } else {
    $('jsonModalContent').value = '[]';
  }
  $('jsonModal').style.display = 'flex';
}

function closeJSONModal() { $('jsonModal').style.display = 'none'; }
function copyJSONModal() {
  const ta = $('jsonModalContent');
  ta.select();
  navigator.clipboard.writeText(ta.value).then(() => {
    alert('JSON copied to clipboard!');
  });
}

function downloadCSV() {
  window.open(window.location.origin + '/api/v1/rates?format=csv', '_blank');
}

window.addEventListener('DOMContentLoaded', () => {
  $('catalogBody').addEventListener('click', (e) => {
    const btn = e.target.closest('.btn-assign');
    if (btn) {
      openAssignModalFor(btn.dataset.resourceType, btn.dataset.sku, btn.dataset.category);
    }
  });

  $('ratesBody').addEventListener('click', (e) => {
    const btn = e.target.closest('.btn-retire');
    if (btn) {
      const id = parseInt(btn.dataset.rateId, 10);
      if (!isNaN(id)) {
        retireRate(id);
      }
    }
  });

  updateTokenBtn();
  loadAll();
});
</script>
</body>
</html>
`
