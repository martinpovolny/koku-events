package api

const indexHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Cost Management &amp; Events — Overview &amp; API Directory</title>
<style>
  :root {
    --bg: #f4f6f9; --card: #ffffff; --border: #dde3ea; --text: #1a1a2e;
    --muted: #6b7280; --accent: #cc0000; --accent2: #a30000;
    --green: #16a34a; --amber: #d97706; --blue: #1d4ed8; --purple: #7c3aed;
    --teal: #0f766e; --method-get: #0284c7; --method-post: #16a34a;
    --method-put: #d97706; --method-delete: #dc2626;
  }
  * { box-sizing: border-box; margin: 0; padding: 0; }
  body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
         background: var(--bg); color: var(--text); min-height: 100vh; line-height: 1.5; }

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

  /* Container */
  .container { max-width: 1280px; margin: 0 auto; padding: 2rem; }

  /* Hero */
  .hero { background: white; border: 1px solid var(--border); border-radius: 10px;
          padding: 1.75rem 2rem; margin-bottom: 2rem; box-shadow: 0 1px 3px rgba(0,0,0,0.04);
          display: flex; justify-content: space-between; align-items: center; }
  .hero-text h1 { font-size: 1.45rem; font-weight: 700; margin-bottom: 0.3rem; }
  .hero-text p { color: var(--muted); font-size: 0.9rem; }
  .hero-status { display: flex; align-items: center; gap: 0.6rem; font-size: 0.85rem; font-weight: 600;
                 padding: 0.5rem 1rem; border-radius: 9999px; background: #ecfdf5; color: var(--green); border: 1px solid #a7f3d0; }
  .status-dot { width: 8px; height: 8px; border-radius: 50%; background: var(--green); }

  /* Section Title */
  .section-title { font-size: 1.1rem; font-weight: 700; margin-bottom: 1rem; display: flex; align-items: center; gap: 0.5rem; }
  .section-title span.count { font-size: 0.8rem; font-weight: normal; color: var(--muted); background: #e5e7eb; padding: 0.1rem 0.45rem; border-radius: 9999px; }

  /* Tool Cards Grid */
  .tools-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(360px, 1fr)); gap: 1.25rem; margin-bottom: 2.5rem; }
  .tool-card { background: white; border: 1px solid var(--border); border-radius: 10px; padding: 1.5rem;
               box-shadow: 0 1px 3px rgba(0,0,0,0.05); transition: transform 0.15s, box-shadow 0.15s; display: flex; flex-direction: column; }
  .tool-card:hover { transform: translateY(-2px); box-shadow: 0 6px 16px rgba(0,0,0,0.08); }
  .tool-icon { font-size: 1.6rem; margin-bottom: 0.75rem; }
  .tool-title { font-size: 1.1rem; font-weight: 700; margin-bottom: 0.4rem; display: flex; align-items: center; justify-content: space-between; }
  .tool-title a { color: var(--text); text-decoration: none; }
  .tool-title a:hover { color: var(--accent); }
  .tool-desc { color: var(--muted); font-size: 0.85rem; margin-bottom: 1.25rem; flex: 1; }
  .tool-footer { display: flex; justify-content: space-between; align-items: center; padding-top: 1rem; border-top: 1px solid #f3f4f6; }
  .tool-path { font-family: "SF Mono", "Fira Code", monospace; font-size: 0.8rem; color: var(--muted); background: #f3f4f6; padding: 0.2rem 0.5rem; border-radius: 4px; }
  .btn-open { background: var(--accent); color: white; text-decoration: none; padding: 0.4rem 0.9rem; border-radius: 5px; font-size: 0.82rem; font-weight: 600; }
  .btn-open:hover { background: var(--accent2); }

  /* API Tables */
  .api-card { background: white; border: 1px solid var(--border); border-radius: 10px; overflow: hidden; margin-bottom: 1.75rem; box-shadow: 0 1px 3px rgba(0,0,0,0.04); }
  .api-header { padding: 0.85rem 1.25rem; background: #fafbfc; border-bottom: 1px solid var(--border); font-weight: 600; font-size: 0.88rem; color: var(--text); display: flex; justify-content: space-between; }
  table { width: 100%; border-collapse: collapse; font-size: 0.86rem; }
  th { padding: 0.65rem 1.25rem; text-align: left; font-size: 0.74rem; text-transform: uppercase; letter-spacing: 0.05em; color: var(--muted); border-bottom: 1px solid var(--border); font-weight: 600; background: #ffffff; }
  td { padding: 0.65rem 1.25rem; border-bottom: 1px solid #f3f4f6; vertical-align: middle; }
  tr:last-child td { border-bottom: none; }
  tr:hover td { background: #fafbfc; }

  /* Method Badges */
  .method { display: inline-block; padding: 0.15rem 0.45rem; border-radius: 4px; font-size: 0.72rem; font-weight: 700; font-family: "SF Mono", "Fira Code", monospace; min-width: 48px; text-align: center; }
  .method-get { background: #e0f2fe; color: #0369a1; }
  .method-post { background: #dcfce7; color: #15803d; }
  .method-put { background: #fef3c7; color: #b45309; }
  .method-delete { background: #fee2e2; color: #b91c1c; }

  .endpoint-path { font-family: "SF Mono", "Fira Code", monospace; font-weight: 600; color: var(--text); }
  .endpoint-desc { color: var(--muted); font-size: 0.82rem; }
  .endpoint-badge { font-size: 0.7rem; background: #e5e7eb; padding: 0.1rem 0.4rem; border-radius: 3px; color: var(--text); margin-left: 0.4rem; }

  /* Modal */
  .modal-overlay { position: fixed; inset: 0; background: rgba(0,0,0,0.45);
                   display: flex; align-items: center; justify-content: center; z-index: 100; }
  .modal { background: white; border-radius: 10px; padding: 1.75rem; width: 500px;
           max-width: 95vw; box-shadow: 0 20px 60px rgba(0,0,0,0.2); }
  .modal h2 { font-size: 1.15rem; margin-bottom: 0.4rem; }
  .modal p { font-size: 0.85rem; color: var(--muted); margin-bottom: 1rem; }
  .modal textarea { width: 100%; height: 90px; font-family: "SF Mono", "Fira Code", monospace; font-size: 0.8rem; border: 1px solid var(--border); border-radius: 5px; padding: 0.5rem; }
  .modal-btns { display: flex; gap: 0.5rem; justify-content: flex-end; margin-top: 1rem; }
  .modal-btns button { padding: 0.45rem 1.1rem; border-radius: 5px; border: none; cursor: pointer; font-size: 0.85rem; font-weight: 500; }
  .btn-primary { background: var(--accent); color: white; }
  .btn-secondary { background: #f0f4f8; color: var(--text); }
</style>
</head>
<body>

<div class="header">
  <div class="header-left">
    <span class="header-logo">Cost Management</span>
    <span class="header-sub">Overview &amp; Directory</span>
    <nav class="header-nav">
      <a href="/ui" class="active">Overview</a>
      <a href="/ui/rates">Catalog &amp; Rates</a>
      <a href="/ui/reports">Reports</a>
      <a href="/ui/dashboard">Diagnostics</a>
    </nav>
  </div>
  <div class="header-right">
    <button class="token-btn" id="tokenBtn" onclick="showTokenModal()">Token</button>
  </div>
</div>

<div class="container">
  <div class="hero">
    <div class="hero-text">
      <h1>Koku Cost Management &amp; Events Service</h1>
      <p>Real-time rating engine, machine sizing catalog, multi-tenant billing, and CloudEvents pipeline.</p>
    </div>
    <div class="hero-status" id="serviceStatus">
      <div class="status-dot"></div>
      <span id="statusLabel">Ready</span>
    </div>
  </div>

  <div class="section-title">
    Interactive Web Tools
    <span class="count">3 Applications</span>
  </div>

  <div class="tools-grid">
    <!-- Card 1: Catalog & Rates -->
    <div class="tool-card">
      <div class="tool-icon">🏷️</div>
      <div class="tool-title">
        <a href="/ui/rates">Catalog &amp; Rate Assignment</a>
      </div>
      <div class="tool-desc">
        Browse synchronized machine instance types (sizing specs) and OSAC catalog offerings. Assign hourly rates ($/hr) or direct unit rates, inspect the rating fallback index, and export rate cards as CSV.
      </div>
      <div class="tool-footer">
        <span class="tool-path">/ui/rates</span>
        <a href="/ui/rates" class="btn-open">Open Tool →</a>
      </div>
    </div>

    <!-- Card 2: Reports -->
    <div class="tool-card">
      <div class="tool-icon">📊</div>
      <div class="tool-title">
        <a href="/ui/reports">Manager Cost Reports</a>
      </div>
      <div class="tool-desc">
        Interactive cost analysis by organization and resource type. Features monthly/daily period filtering, infrastructure vs. AI inference cost split, and tenant quota tracking.
      </div>
      <div class="tool-footer">
        <span class="tool-path">/ui/reports</span>
        <a href="/ui/reports" class="btn-open">Open Tool →</a>
      </div>
    </div>

    <!-- Card 3: Diagnostics Dashboard -->
    <div class="tool-card">
      <div class="tool-icon">⚙️</div>
      <div class="tool-title">
        <a href="/ui/dashboard">Diagnostics Dashboard</a>
      </div>
      <div class="tool-desc">
        Real-time pipeline diagnostics. Monitor event ingestion throughput, active billable inventory, quota headroom, prepaid wallet balances, and runtime reconciler configuration.
      </div>
      <div class="tool-footer">
        <span class="tool-path">/ui/dashboard</span>
        <a href="/ui/dashboard" class="btn-open">Open Tool →</a>
      </div>
    </div>
  </div>

  <div class="section-title">
    REST API Directory
    <span class="count">OpenAPI 3.0.3</span>
  </div>

  <!-- API Group: Catalog & Rates -->
  <div class="api-card">
    <div class="api-header">
      <span>Catalog &amp; Rates APIs</span>
      <span style="font-weight:400;color:var(--muted);font-size:0.8rem">Machine types, catalog templates, rate cards</span>
    </div>
    <table>
      <thead>
        <tr>
          <th style="width:90px">Method</th>
          <th style="width:260px">Path</th>
          <th>Description</th>
          <th style="width:120px">Format</th>
        </tr>
      </thead>
      <tbody>
        <tr>
          <td><span class="method method-get">GET</span></td>
          <td class="endpoint-path">/api/v1/catalog</td>
          <td class="endpoint-desc">List synchronized machine instance types (cores, RAM) and service catalog templates</td>
          <td><span class="endpoint-badge">JSON</span></td>
        </tr>
        <tr>
          <td><span class="method method-get">GET</span></td>
          <td class="endpoint-path">/api/v1/rates</td>
          <td class="endpoint-desc">List active rate cards with 4-way fallback matching. Filter by <code>?tenant_id=...</code></td>
          <td><span class="endpoint-badge">JSON</span> <span class="endpoint-badge">CSV</span></td>
        </tr>
        <tr>
          <td><span class="method method-post">POST</span></td>
          <td class="endpoint-path">/api/v1/rates</td>
          <td class="endpoint-desc">Create or supersede a rate card definition for an instance type / meter</td>
          <td><span class="endpoint-badge">JSON</span></td>
        </tr>
        <tr>
          <td><span class="method method-delete">DELETE</span></td>
          <td class="endpoint-path">/api/v1/rates/{id}</td>
          <td class="endpoint-desc">Soft-delete (retire) a rate card by setting <code>effective_to = NOW()</code></td>
          <td><span class="endpoint-badge">204 No Content</span></td>
        </tr>
      </tbody>
    </table>
  </div>

  <!-- API Group: Cost Reports -->
  <div class="api-card">
    <div class="api-header">
      <span>Reporting &amp; Breakdown APIs</span>
      <span style="font-weight:400;color:var(--muted);font-size:0.8rem">Koku-compatible cost aggregation</span>
    </div>
    <table>
      <thead>
        <tr>
          <th style="width:90px">Method</th>
          <th style="width:260px">Path</th>
          <th>Description</th>
          <th style="width:120px">Format</th>
        </tr>
      </thead>
      <tbody>
        <tr>
          <td><span class="method method-get">GET</span></td>
          <td class="endpoint-path">/api/v1/reports/costs</td>
          <td class="endpoint-desc">Aggregated cost reports. Parameters: <code>group_by=tenant|resource_type</code>, <code>resolution=daily|monthly</code></td>
          <td><span class="endpoint-badge">JSON</span> <span class="endpoint-badge">CSV</span></td>
        </tr>
        <tr>
          <td><span class="method method-get">GET</span></td>
          <td class="endpoint-path">/api/v1/reports/breakdown</td>
          <td class="endpoint-desc">Detailed line-item cost breakdown with tier calculations and wallet deductions</td>
          <td><span class="endpoint-badge">JSON</span> <span class="endpoint-badge">CSV</span></td>
        </tr>
        <tr>
          <td><span class="method method-get">GET</span></td>
          <td class="endpoint-path">/api/v1/reports/summary</td>
          <td class="endpoint-desc">High-level pipeline counts: total costs, active resources, event volume</td>
          <td><span class="endpoint-badge">JSON</span></td>
        </tr>
      </tbody>
    </table>
  </div>

  <!-- API Group: Quotas & Budgets -->
  <div class="api-card">
    <div class="api-header">
      <span>Quotas &amp; Budgets APIs</span>
      <span style="font-weight:400;color:var(--muted);font-size:0.8rem">Multi-tenant limits &amp; boundary enforcement</span>
    </div>
    <table>
      <thead>
        <tr>
          <th style="width:90px">Method</th>
          <th style="width:260px">Path</th>
          <th>Description</th>
          <th style="width:120px">Format</th>
        </tr>
      </thead>
      <tbody>
        <tr>
          <td><span class="method method-get">GET</span></td>
          <td class="endpoint-path">/api/v1/quotas</td>
          <td class="endpoint-desc">List all defined quotas and budgets, optionally with real-time consumption status</td>
          <td><span class="endpoint-badge">JSON</span></td>
        </tr>
        <tr>
          <td><span class="method method-post">POST</span></td>
          <td class="endpoint-path">/api/v1/quotas</td>
          <td class="endpoint-desc">Create a metric limit or monetary budget with enforcement policies</td>
          <td><span class="endpoint-badge">JSON</span></td>
        </tr>
        <tr>
          <td><span class="method method-get">GET</span></td>
          <td class="endpoint-path">/api/v1/quotas/{tenant_id}</td>
          <td class="endpoint-desc">Get live quota consumption percentage and status for a specific tenant</td>
          <td><span class="endpoint-badge">JSON</span></td>
        </tr>
        <tr>
          <td><span class="method method-put">PUT</span></td>
          <td class="endpoint-path">/api/v1/quotas/{id}</td>
          <td class="endpoint-desc">Update an existing quota limit or period</td>
          <td><span class="endpoint-badge">JSON</span></td>
        </tr>
        <tr>
          <td><span class="method method-delete">DELETE</span></td>
          <td class="endpoint-path">/api/v1/quotas/{id}</td>
          <td class="endpoint-desc">Soft-delete a quota by ID</td>
          <td><span class="endpoint-badge">204 No Content</span></td>
        </tr>
      </tbody>
    </table>
  </div>

  <!-- API Group: Wallets -->
  <div class="api-card">
    <div class="api-header">
      <span>Prepaid Wallets &amp; Ledger APIs</span>
      <span style="font-weight:400;color:var(--muted);font-size:0.8rem">Prepaid balance management &amp; audit ledger</span>
    </div>
    <table>
      <thead>
        <tr>
          <th style="width:90px">Method</th>
          <th style="width:260px">Path</th>
          <th>Description</th>
          <th style="width:120px">Format</th>
        </tr>
      </thead>
      <tbody>
        <tr>
          <td><span class="method method-get">GET</span></td>
          <td class="endpoint-path">/api/v1/wallets/{id}</td>
          <td class="endpoint-desc">Retrieve wallet balance, currency, and lifetime credit/debit summary</td>
          <td><span class="endpoint-badge">JSON</span></td>
        </tr>
        <tr>
          <td><span class="method method-post">POST</span></td>
          <td class="endpoint-path">/api/v1/wallets</td>
          <td class="endpoint-desc">Create a new prepaid wallet for a tenant or project</td>
          <td><span class="endpoint-badge">JSON</span></td>
        </tr>
        <tr>
          <td><span class="method method-post">POST</span></td>
          <td class="endpoint-path">/api/v1/wallets/{id}/top-ups</td>
          <td class="endpoint-desc">Add prepaid credit funds to an active wallet</td>
          <td><span class="endpoint-badge">JSON</span></td>
        </tr>
        <tr>
          <td><span class="method method-post">POST</span></td>
          <td class="endpoint-path">/api/v1/wallets/{id}/adjustments</td>
          <td class="endpoint-desc">Post an administrative debit or credit adjustment</td>
          <td><span class="endpoint-badge">JSON</span></td>
        </tr>
        <tr>
          <td><span class="method method-get">GET</span></td>
          <td class="endpoint-path">/api/v1/wallets/{id}/ledger</td>
          <td class="endpoint-desc">Audit trail of all ledger transactions (deductions, top-ups, adjustments)</td>
          <td><span class="endpoint-badge">JSON</span></td>
        </tr>
      </tbody>
    </table>
  </div>

  <!-- API Group: Events & Operations -->
  <div class="api-card">
    <div class="api-header">
      <span>Event Ingestion &amp; Operations APIs</span>
      <span style="font-weight:400;color:var(--muted);font-size:0.8rem">CloudEvents ingestion &amp; system health</span>
    </div>
    <table>
      <thead>
        <tr>
          <th style="width:90px">Method</th>
          <th style="width:260px">Path</th>
          <th>Description</th>
          <th style="width:120px">Format</th>
        </tr>
      </thead>
      <tbody>
        <tr>
          <td><span class="method method-post">POST</span></td>
          <td class="endpoint-path">/api/v1/events/batch</td>
          <td class="endpoint-desc">Atomic ingestion of canonical OSAC CloudEvents (VMaaS, CaaS, MaaS)</td>
          <td><span class="endpoint-badge">JSON / CloudEvents</span></td>
        </tr>
        <tr>
          <td><span class="method method-get">GET</span></td>
          <td class="endpoint-path">/healthz</td>
          <td class="endpoint-desc">Kubernetes liveness probe (exempt from authentication)</td>
          <td><span class="endpoint-badge">200 OK</span></td>
        </tr>
        <tr>
          <td><span class="method method-get">GET</span></td>
          <td class="endpoint-path">/readyz</td>
          <td class="endpoint-desc">Kubernetes readiness probe (verifies database connectivity)</td>
          <td><span class="endpoint-badge">200 OK</span></td>
        </tr>
        <tr>
          <td><span class="method method-get">GET</span></td>
          <td class="endpoint-path">/api/v1/debug/config</td>
          <td class="endpoint-desc">Sanitized runtime configuration and reconciler entity filter diagnostics</td>
          <td><span class="endpoint-badge">JSON</span></td>
        </tr>
        <tr>
          <td><span class="method method-post">POST</span></td>
          <td class="endpoint-path">/api/v1/reconcile</td>
          <td class="endpoint-desc">Trigger an immediate asynchronous OSAC reconciliation sweep</td>
          <td><span class="endpoint-badge">202 Accepted</span></td>
        </tr>
      </tbody>
    </table>
  </div>
</div>

<!-- Token Modal -->
<div class="modal-overlay" id="tokenModal" style="display:none">
  <div class="modal">
    <h2>API Authentication Token</h2>
    <p>Set a JWT Bearer token to authorize write and read operations across all UI tools. The token is stored locally in your browser (<code>koku_report_token</code>).</p>
    <textarea id="tokenInput" placeholder="eyJhbGciOi..."></textarea>
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
  updateTokenBtn(); hideTokenModal();
}
function clearToken() { localStorage.removeItem(TOKEN_KEY); updateTokenBtn(); hideTokenModal(); }

async function checkHealth() {
  try {
    const res = await fetch(window.location.origin + '/readyz');
    const statusEl = $('serviceStatus');
    const labelEl = $('statusLabel');
    if (res.ok) {
      statusEl.style.background = '#ecfdf5';
      statusEl.style.color = '#16a34a';
      statusEl.style.borderColor = '#a7f3d0';
      labelEl.textContent = 'Ready (Database Connected)';
    } else {
      statusEl.style.background = '#fef2f2';
      statusEl.style.color = '#dc2626';
      statusEl.style.borderColor = '#fecaca';
      labelEl.textContent = 'Degraded (HTTP ' + res.status + ')';
    }
  } catch (e) {
    const statusEl = $('serviceStatus');
    const labelEl = $('statusLabel');
    statusEl.style.background = '#fffbeb';
    statusEl.style.color = '#d97706';
    statusEl.style.borderColor = '#fde68a';
    labelEl.textContent = 'Offline / Connecting';
  }
}

window.addEventListener('DOMContentLoaded', () => {
  updateTokenBtn();
  checkHealth();
});
</script>
</body>
</html>
`
