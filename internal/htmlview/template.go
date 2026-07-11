package htmlview

import (
	"fmt"
	"html"
	"net/http"
	"strings"

	"gin-sqlserver-api/internal/models"

	"github.com/gin-gonic/gin"
)

// WantsHTML returns true when the caller is a regular browser expecting an HTML
// page rather than a programmatic client that wants JSON.  Streamlit, curl with
// explicit Accept headers, and any request that does NOT include "text/html"
// in its Accept header will still receive JSON.
func WantsHTML(c *gin.Context) bool {
	accept := c.GetHeader("Accept")
	if strings.Contains(accept, "text/html") {
		return true
	}
	return false
}

// RenderClients writes the professional HTML banking page for a list of clients.
func RenderClients(c *gin.Context, clients []models.Client, nodeName string) {
	rows := buildTableRows(clients)
	page := fmt.Sprintf(pageTemplate, html.EscapeString(nodeName), len(clients), rows)
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(page))
}

func buildTableRows(clients []models.Client) string {
	if len(clients) == 0 {
		return `<tr><td colspan="6" style="text-align:center;color:#94a3b8;padding:40px 0;font-size:15px;">No clients found in RAM store.</td></tr>`
	}
	var sb strings.Builder
	for _, cl := range clients {
		sb.WriteString(fmt.Sprintf(
			`<tr>
				<td>%d</td>
				<td>%s</td>
				<td>%s</td>
				<td>%s</td>
				<td>%s</td>
				<td class="balance">%.2f EGP</td>
			</tr>`,
			cl.ID,
			html.EscapeString(cl.Name),
			html.EscapeString(cl.NationalID),
			html.EscapeString(cl.Phone),
			html.EscapeString(cl.Email),
			cl.Balance,
		))
	}
	return sb.String()
}

const pageTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Distributed Banking System — %s</title>
<meta name="description" content="In-memory distributed banking client database with real-time replication across five nodes.">
<link rel="preconnect" href="https://fonts.googleapis.com">
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700;800;900&display=swap" rel="stylesheet">
<style>
*,*::before,*::after{box-sizing:border-box;margin:0;padding:0}
:root{
  --bg-primary:#07111f;
  --bg-card:rgba(15,23,42,.92);
  --bg-row-hover:rgba(56,189,248,.06);
  --border:rgba(148,163,184,.14);
  --accent:#38bdf8;
  --accent-glow:rgba(56,189,248,.18);
  --text-primary:#f1f5f9;
  --text-secondary:#94a3b8;
  --text-muted:#64748b;
  --green:#34d399;
  --green-bg:rgba(52,211,153,.12);
  --purple:#a78bfa;
}
body{
  font-family:'Inter',system-ui,-apple-system,sans-serif;
  background:
    radial-gradient(ellipse at 20%% 0%%,rgba(56,189,248,.10),transparent 50%%),
    radial-gradient(ellipse at 80%% 0%%,rgba(167,139,250,.08),transparent 50%%),
    var(--bg-primary);
  color:var(--text-primary);
  min-height:100vh;
  line-height:1.5;
}
.wrapper{
  max-width:1280px;
  margin:0 auto;
  padding:32px 24px 64px;
}

/* ── Header ── */
.header{
  display:flex;
  align-items:center;
  justify-content:space-between;
  flex-wrap:wrap;
  gap:16px;
  margin-bottom:32px;
}
.logo{
  display:flex;align-items:center;gap:14px;
}
.logo-icon{
  width:48px;height:48px;
  background:linear-gradient(135deg,var(--accent),var(--purple));
  border-radius:14px;
  display:flex;align-items:center;justify-content:center;
  font-size:22px;font-weight:900;color:#fff;
  box-shadow:0 8px 24px var(--accent-glow);
}
.logo-text h1{
  font-size:22px;font-weight:800;letter-spacing:-.3px;
  background:linear-gradient(135deg,#fff,var(--accent));
  -webkit-background-clip:text;-webkit-text-fill-color:transparent;
}
.logo-text p{
  font-size:13px;color:var(--text-muted);margin-top:2px;
}
.header-badges{display:flex;gap:10px;flex-wrap:wrap;}
.badge{
  padding:7px 16px;
  border-radius:999px;
  font-size:12px;font-weight:700;
  letter-spacing:.3px;
  border:1px solid;
}
.badge-node{
  background:rgba(56,189,248,.08);
  color:var(--accent);
  border-color:rgba(56,189,248,.22);
}
.badge-ram{
  background:var(--green-bg);
  color:var(--green);
  border-color:rgba(52,211,153,.25);
}
.badge-count{
  background:rgba(167,139,250,.10);
  color:var(--purple);
  border-color:rgba(167,139,250,.25);
}

/* ── Stats Strip ── */
.stats{
  display:grid;
  grid-template-columns:repeat(auto-fit,minmax(200px,1fr));
  gap:16px;
  margin-bottom:32px;
}
.stat-card{
  background:var(--bg-card);
  border:1px solid var(--border);
  border-radius:16px;
  padding:20px 22px;
  position:relative;
  overflow:hidden;
  transition:border-color .25s,box-shadow .25s;
}
.stat-card:hover{
  border-color:rgba(56,189,248,.28);
  box-shadow:0 12px 40px rgba(0,0,0,.3);
}
.stat-card::before{
  content:'';position:absolute;top:0;left:0;right:0;height:3px;
  background:linear-gradient(90deg,var(--accent),var(--purple));
  opacity:.6;border-radius:16px 16px 0 0;
}
.stat-label{font-size:12px;color:var(--text-muted);text-transform:uppercase;letter-spacing:.8px;font-weight:600;}
.stat-value{font-size:28px;font-weight:800;margin-top:6px;color:var(--text-primary);}
.stat-sub{font-size:12px;color:var(--text-secondary);margin-top:4px;}

/* ── Table Card ── */
.table-card{
  background:var(--bg-card);
  border:1px solid var(--border);
  border-radius:20px;
  overflow:hidden;
  box-shadow:0 20px 60px rgba(0,0,0,.28);
}
.table-header{
  padding:22px 28px;
  border-bottom:1px solid var(--border);
  display:flex;align-items:center;justify-content:space-between;flex-wrap:wrap;gap:12px;
}
.table-header h2{
  font-size:18px;font-weight:700;
}
.table-header .hint{
  font-size:12px;color:var(--text-muted);
  background:rgba(100,116,139,.12);
  padding:5px 12px;border-radius:8px;
}
table{
  width:100%%;border-collapse:collapse;
}
thead{
  background:rgba(15,23,42,.5);
}
th{
  padding:14px 20px;
  text-align:left;
  font-size:11px;
  text-transform:uppercase;
  letter-spacing:.8px;
  color:var(--text-muted);
  font-weight:700;
  border-bottom:1px solid var(--border);
  white-space:nowrap;
}
td{
  padding:16px 20px;
  font-size:14px;
  border-bottom:1px solid var(--border);
  color:var(--text-secondary);
  transition:background .15s;
}
tr:hover td{
  background:var(--bg-row-hover);
  color:var(--text-primary);
}
tr:last-child td{border-bottom:none;}
td.balance{
  font-weight:700;
  color:var(--green);
  font-variant-numeric:tabular-nums;
}

/* ── Footer ── */
.footer{
  text-align:center;
  margin-top:40px;
  padding:18px;
  font-size:12px;
  color:var(--text-muted);
}
.footer a{color:var(--accent);text-decoration:none;}
.footer a:hover{text-decoration:underline;}

/* ── Responsive ── */
@media(max-width:768px){
  .wrapper{padding:20px 12px 40px;}
  .header{flex-direction:column;align-items:flex-start;}
  td,th{padding:10px 12px;font-size:13px;}
  .stat-value{font-size:22px;}
}
</style>
</head>
<body>
<div class="wrapper">
  <header class="header">
    <div class="logo">
      <div class="logo-icon">DB</div>
      <div class="logo-text">
        <h1>Distributed Banking System</h1>
        <p>In-Memory RAM Store · Real-Time Replication</p>
      </div>
    </div>
    <div class="header-badges">
      <span class="badge badge-node">Node: %[1]s</span>
      <span class="badge badge-ram">RAM Store</span>
      <span class="badge badge-count">%[2]d Clients</span>
    </div>
  </header>

  <section class="stats">
    <div class="stat-card">
      <div class="stat-label">Total Clients</div>
      <div class="stat-value">%[2]d</div>
      <div class="stat-sub">Stored in RAM</div>
    </div>
    <div class="stat-card">
      <div class="stat-label">Active Node</div>
      <div class="stat-value">%[1]s</div>
      <div class="stat-sub">In-Memory Store</div>
    </div>
    <div class="stat-card">
      <div class="stat-label">Storage Engine</div>
      <div class="stat-value">RAM</div>
      <div class="stat-sub">In-Memory · No Disk</div>
    </div>
    <div class="stat-card">
      <div class="stat-label">Architecture</div>
      <div class="stat-value">5 Nodes</div>
      <div class="stat-sub">Master-Slave Replication</div>
    </div>
  </section>

  <div class="table-card">
    <div class="table-header">
      <h2>Client Records</h2>
      <span class="hint">GET /clients · JSON available via Accept: application/json</span>
    </div>
    <table>
      <thead>
        <tr>
          <th>ID</th>
          <th>Name</th>
          <th>National ID</th>
          <th>Phone</th>
          <th>Email</th>
          <th>Balance</th>
        </tr>
      </thead>
      <tbody>
        %[3]s
      </tbody>
    </table>
  </div>

  <footer class="footer">
    Distributed Database Project · University Demo · In-Memory RAM Storage · 5-Node Architecture
  </footer>
</div>
</body>
</html>`
