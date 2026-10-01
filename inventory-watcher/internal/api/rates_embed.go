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
  .kpi.unpriced .kpi-value { color: var(--amber); }

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
  .badge-retired { background: #fee2e2; color: var(--red); }

  .code-text { font-family: "SF Mono", "Fira Code", monospace; font-size: 0.8rem; background: #f3f4f6; padding: 0.15rem 0.35rem; border-radius: 3px; }

  /* Alerts */
  .error-banner { background: #fee2e2; color: var(--red); padding: 0.75rem 1.25rem;
                  border-radius: 6px; margin-bottom: 1rem; font-size: 0.88rem; display: none; }
  .success-banner { background: #dcfce7; color: var(--green); padding: 0.75rem 1.25rem;
                    border-radius: 6px; margin-bottom: 1rem; font-size: 0.88rem; display: none; }

  /* Modals */
  .modal-overlay { position: fixed; inset: 0; background: rgba(0,0,0,0.45);
                   display: flex; align-items: center; justify-content: center; z-index: 100; }
  .modal { background: white; border-radius: 10px; padding: 1.75rem; width: 560px;
           max-width: 95vw; box-shadow: 0 20px 60px rgba(0,0,0,0.2); }
  .modal h2 { font-size: 1.15rem; margin-bottom: 0.4rem; color: var(--text); }
  .modal p { font-size: 0.85rem; color: var(--muted); margin-bottom: 1rem; line-height: 1.5; }
  .form-group { margin-bottom: 1rem; }
  .form-group label { display: block; font-size: 0.8rem; font-weight: 600; color: var(--text); margin-bottom: 0.3rem; }
  .form-group input, .form-group select, .form-group textarea {
    width: 100%; border: 1px solid var(--border); border-radius: 5px;
    padding: 0.5rem 0.75rem; font-size: 0.88rem; color: var(--text); background: white; }
  .form-group .hint { font-size: 0.75rem; color: var(--muted); margin-top: 0.25rem; }
  .form-row { display: grid; grid-template-columns: 1fr 1fr; gap: 0.75rem; }
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
      <a href="/rates" class="active">Catalog &amp; Rates</a>
      <a href="/reports">Reports</a>
      <a href="/debug/dashboard">Diagnostics</a>
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
  <div class="kpi unpriced">
    <div class="kpi-value" id="kUnpriced">—</div>
    <div class="kpi-label">Unpriced Offerings</div>
  </div>
</div>

<div class="content">
  <div id="errorBanner" class="error-banner"></div>
  <div id="successBanner" class="success-banner"></div>

  <div class="tabs">
    <div class="tab active" data-tab="catalog" onclick="switchTab(this)">Catalog &amp; Rate Assignment</div>
    <div class="tab" data-tab="rates" onclick="switchTab(this)">Active Rate Cards (<span id="rateCountTab">0</span>)</div>
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
            <th>Assigned Rate</th>
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
        <span class="subtitle">Direct rate matching index with 4-way fallback</span>
      </div>
      <table>
        <thead>
          <tr>
            <th>ID</th>
            <th>Scope / Tenant</th>
            <th>Resource Type</th>
            <th>SKU / Instance Type</th>
            <th>Meter Name</th>
            <th>Cost Type</th>
            <th class="num">Price / Unit</th>
            <th class="num">Hourly Rate</th>
            <th>Effective</th>
            <th class="num">Actions</th>
          </tr>
        </thead>
        <tbody id="ratesBody">
          <tr><td colspan="10" class="empty">Loading rates…</td></tr>
        </tbody>
      </table>
    </div>
  </div>
</div>

<!-- Assign / Edit Rate Modal -->
<div class="modal-overlay" id="rateModal" style="display:none">
  <div class="modal">
    <h2 id="modalTitle">Assign Rate Card</h2>
    <p>Configure a rate card for catalog sizing or template metering. Active rates take effect immediately in the rating engine.</p>

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
        <label>Instance Type / SKU</label>
        <input type="text" id="rateInstanceType" placeholder="e.g. standard-4-16 (or blank for all)">
        <div class="hint">Matches machine type or catalog template name.</div>
      </div>
    </div>

    <div class="form-row">
      <div class="form-group">
        <label>Meter Name</label>
        <input type="text" id="rateMeterName" placeholder="e.g. vm_uptime_seconds">
        <div class="hint" id="meterHint">Default for VMs: vm_uptime_seconds</div>
      </div>
      <div class="form-group">
        <label>Cost Layer</label>
        <select id="rateCostType">
          <option value="Infrastructure">Infrastructure</option>
          <option value="Supplementary">Supplementary</option>
        </select>
      </div>
    </div>

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

    <div class="form-row">
      <div class="form-group">
        <label>Tenant ID (Optional)</label>
        <input type="text" id="rateTenantId" placeholder="Blank for global default">
      </div>
      <div class="form-group">
        <label>Currency</label>
        <input type="text" id="rateCurrency" value="USD">
      </div>
    </div>

    <div class="form-group">
      <label>Description</label>
      <input type="text" id="rateDescription" placeholder="e.g. Standard 4 vCPU 16GB VM rate ($0.10/hour)">
    </div>

    <div class="modal-btns">
      <button class="btn-secondary" onclick="closeAssignModal()">Cancel</button>
      <button class="btn-primary" onclick="submitRateForm()">Save Rate Card</button>
    </div>
  </div>
</div>

<!-- Token Modal -->
<div class="modal-overlay" id="tokenModal" style="display:none">
  <div class="modal">
    <h2>API Authentication Token</h2>
    <p>Paste an OSAC service token or JWT Bearer token to authorize write and read operations. Stored locally in your browser.</p>
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
  activeTab: 'catalog'
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
  renderAll();
}

async function loadAll() {
  $('errorBanner').style.display = 'none';
  try {
    const [catRes, ratesRes] = await Promise.all([
      api('/api/v1/catalog'),
      api('/api/v1/rates')
    ]);

    if (!catRes.ok) throw new Error('Failed to load catalog: ' + catRes.status);
    if (!ratesRes.ok) throw new Error('Failed to load rates: ' + ratesRes.status);

    const catData = await catRes.json();
    const ratesData = await ratesRes.json();

    state.catalogItems = catData.catalog_items || [];
    state.instanceTypes = catData.instance_types || [];
    state.rates = ratesData.rates || [];

    $('lastUpdated').textContent = 'Synced ' + new Date().toLocaleTimeString();
    renderAll();
  } catch (err) {
    showBanner('error', err.message);
  }
}

function findMatchingRate(resourceType, instanceType) {
  // Check exact instance_type + resource_type match first (global or any tenant)
  const exact = state.rates.find(r => (!r.effective_to || new Date(r.effective_to) > new Date()) &&
    r.resource_type.toLowerCase() === resourceType.toLowerCase() &&
    r.instance_type === instanceType);
  if (exact) return exact;

  // Fallback to resource_type default
  return state.rates.find(r => (!r.effective_to || new Date(r.effective_to) > new Date()) &&
    r.resource_type.toLowerCase() === resourceType.toLowerCase() &&
    (!r.instance_type || r.instance_type === ''));
}

function formatPriceDisplay(rate) {
  if (!rate) return '<span class="badge badge-unpriced">Unpriced</span>';
  const price = parseFloat(rate.price_per_unit) || 0;
  if (rate.meter_name && rate.meter_name.includes('seconds')) {
    const hourly = price * 3600;
    return '<span class="badge badge-priced">$' + hourly.toFixed(4) + ' / hr</span>' +
           ' <span style="font-size:0.75rem;color:var(--muted)">($' + price.toFixed(8) + '/s)</span>';
  }
  return '<span class="badge badge-priced">$' + price.toFixed(6) + ' / ' + (rate.currency || 'USD') + '</span>';
}

function renderAll() {
  const query = $('filterSearch').value.toLowerCase().trim();
  const categoryFilter = $('filterType').value;
  const pricingFilter = $('filterPricing').value;

  // Count active rates
  const now = new Date();
  const activeRates = state.rates.filter(r => !r.effective_to || new Date(r.effective_to) > now);

  $('kInstances').textContent = state.instanceTypes.length;
  $('kCatalog').textContent = state.catalogItems.length;
  $('kRates').textContent = activeRates.length;
  $('rateCountTab').textContent = activeRates.length;

  // Build unified catalog entries
  const unified = [];

  for (const it of state.instanceTypes) {
    const rate = findMatchingRate('compute_instance', it.name);
    unified.push({
      category: 'instance_type',
      categoryLabel: 'Machine Type',
      resourceType: 'compute_instance',
      sku: it.name,
      id: it.instance_type_id,
      title: it.name,
      specs: (it.cores || 0) + ' vCPU, ' + (it.memory_gib || 0) + ' GiB RAM',
      status: it.state || 'active',
      rate: rate
    });
  }

  for (const ci of state.catalogItems) {
    let resType = 'compute_instance';
    let catLabel = 'Compute Offering';
    if (ci.item_type && ci.item_type.includes('cluster')) { resType = 'cluster'; catLabel = 'Cluster Offering'; }
    else if (ci.item_type && ci.item_type.includes('bare_metal')) { resType = 'bare_metal'; catLabel = 'Bare Metal Offering'; }
    else if (ci.item_type && ci.item_type.includes('model')) { resType = 'model'; catLabel = 'Model Offering'; }

    const rate = findMatchingRate(resType, ci.name) || findMatchingRate(resType, ci.catalog_item_id);
    unified.push({
      category: resType,
      categoryLabel: catLabel,
      resourceType: resType,
      sku: ci.name,
      id: ci.catalog_item_id,
      title: ci.title || ci.name,
      specs: ci.description || ('Template: ' + (ci.template || '—')),
      status: ci.published ? 'published' : 'draft',
      rate: rate
    });
  }

  // Calculate unpriced count
  const unpricedCount = unified.filter(u => !u.rate).length;
  $('kUnpriced').textContent = unpricedCount;

  // Filter unified catalog
  const filtered = unified.filter(item => {
    if (categoryFilter && item.category !== categoryFilter) return false;
    if (pricingFilter === 'priced' && !item.rate) return false;
    if (pricingFilter === 'unpriced' && item.rate) return false;
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

      const meterText = item.rate ? '<span class="code-text">' + item.rate.meter_name + '</span>' : '<span style="color:var(--muted)">—</span>';
      const actionText = item.rate ? 'Edit Rate' : 'Assign Rate';

      return '<tr>' +
        '<td><span class="badge ' + badgeClass + '">' + item.categoryLabel + '</span></td>' +
        '<td><span class="code-text" style="font-weight:600">' + item.sku + '</span></td>' +
        '<td><div><strong>' + item.title + '</strong></div><div style="font-size:0.78rem;color:var(--muted)">' + item.specs + '</div></td>' +
        '<td><span style="font-size:0.8rem;text-transform:capitalize">' + item.status + '</span></td>' +
        '<td>' + formatPriceDisplay(item.rate) + '</td>' +
        '<td>' + meterText + '</td>' +
        '<td class="num"><button class="btn-sm primary" onclick="openAssignModalFor(\'' + item.resourceType + '\', \'' + item.sku + '\')">' + actionText + '</button></td>' +
      '</tr>';
    }).join('');
  }

  // Render Rates Table
  const ratesBody = $('ratesBody');
  if (activeRates.length === 0) {
    ratesBody.innerHTML = '<tr><td colspan="10" class="empty">No active rates configured.</td></tr>';
  } else {
    ratesBody.innerHTML = activeRates.map(r => {
      const price = parseFloat(r.price_per_unit) || 0;
      const isUptime = r.meter_name && r.meter_name.includes('seconds');
      const hourly = isUptime ? '$' + (price * 3600).toFixed(4) + '/hr' : '—';
      const tenant = r.tenant_id ? '<span class="code-text">' + r.tenant_id + '</span>' : '<span class="badge badge-infra">Global</span>';
      const costBadge = r.cost_type === 'Supplementary' ? 'badge-supp' : 'badge-infra';
      const effFrom = r.effective_from ? new Date(r.effective_from).toLocaleDateString() : '—';

      return '<tr>' +
        '<td>' + r.id + '</td>' +
        '<td>' + tenant + '</td>' +
        '<td><span class="code-text">' + r.resource_type + '</span></td>' +
        '<td><span class="code-text">' + (r.instance_type || '*(any)*') + '</span></td>' +
        '<td><span class="code-text">' + r.meter_name + '</span></td>' +
        '<td><span class="badge ' + costBadge + '">' + r.cost_type + '</span></td>' +
        '<td class="num">$' + price.toFixed(8) + '</td>' +
        '<td class="num">' + hourly + '</td>' +
        '<td>' + effFrom + '</td>' +
        '<td class="num"><button class="btn-sm" style="color:var(--red);border-color:#fca5a5" onclick="retireRate(' + r.id + ')">Retire</button></td>' +
      '</tr>';
    }).join('');
  }
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

function openAssignModal() {
  $('modalTitle').textContent = 'Assign Rate Card';
  $('rateResourceType').value = 'compute_instance';
  $('rateInstanceType').value = '';
  $('rateMeterName').value = 'vm_uptime_seconds';
  $('rateCostType').value = 'Infrastructure';
  $('ratePriceMode').value = 'hourly';
  $('ratePriceInput').value = '0.10';
  $('rateTenantId').value = '';
  $('rateDescription').value = '';
  onResourceTypeChange();
  $('rateModal').style.display = 'flex';
}

function openAssignModalFor(resourceType, sku) {
  openAssignModal();
  $('modalTitle').textContent = 'Assign Rate: ' + sku;
  $('rateResourceType').value = resourceType;
  $('rateInstanceType').value = sku;
  onResourceTypeChange();
  $('rateDescription').value = 'Rate for ' + sku;
}

function closeAssignModal() { $('rateModal').style.display = 'none'; }

async function submitRateForm() {
  const resourceType = $('rateResourceType').value.trim();
  const instanceType = $('rateInstanceType').value.trim();
  const meterName = $('rateMeterName').value.trim();
  const costType = $('rateCostType').value;
  const currency = $('rateCurrency').value.trim() || 'USD';
  const description = $('rateDescription').value.trim();
  const pricePerUnit = $('ratePricePerUnit').value.trim();
  const tenantId = $('rateTenantId').value.trim();

  if (!resourceType || !meterName) {
    alert('Resource Type and Meter Name are required.');
    return;
  }
  if (!pricePerUnit || parseFloat(pricePerUnit) < 0) {
    alert('Valid price is required.');
    return;
  }

  const payload = {
    resource_type: resourceType,
    instance_type: instanceType,
    meter_name: meterName,
    cost_type: costType,
    price_per_unit: pricePerUnit,
    currency: currency,
    description: description,
    tier_mode: 'per_event'
  };
  if (tenantId) payload.tenant_id = tenantId;

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

function downloadCSV() {
  window.open(window.location.origin + '/api/v1/rates?format=csv', '_blank');
}

window.addEventListener('DOMContentLoaded', () => {
  updateTokenBtn();
  loadAll();
});
</script>
</body>
</html>
`
