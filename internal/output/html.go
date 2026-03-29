package output

import (
	"fmt"
	"html/template"
	"io"
	"strings"
	"time"

	"github.com/themark007/knew/internal/k8s"
)

// ReportData bundles everything the HTML template needs.
type ReportData struct {
	GeneratedAt   string
	Namespace     string
	AllNamespaces bool
	Pods          []k8s.PodInfo
	Services      []k8s.ServiceInfo
	Policies      []k8s.NetworkPolicyInfo
	Ingresses     []k8s.IngressInfo
	Audits        []k8s.NamespaceAudit
	ASCIIGraph    string
	DotGraph      string
	MermaidGraph  string
	AIAnalysis    string
}

// RenderHTML writes a self-contained HTML report to w.
func RenderHTML(w io.Writer, data ReportData) error {
	if data.GeneratedAt == "" {
		data.GeneratedAt = time.Now().Format("2006-01-02 15:04:05 UTC")
	}
	t, err := template.New("report").Funcs(template.FuncMap{
		"lower": strings.ToLower,
		"labelsStr": func(m map[string]string) string {
			if len(m) == 0 {
				return "-"
			}
			var parts []string
			for k, v := range m {
				parts = append(parts, k+"="+v)
			}
			return strings.Join(parts, ", ")
		},
		"portList": func(ports []k8s.PortInfo) string {
			if len(ports) == 0 {
				return "-"
			}
			parts := make([]string, len(ports))
			for i, p := range ports {
				parts[i] = fmt.Sprintf("%d/%s", p.Port, p.Protocol)
			}
			return strings.Join(parts, ", ")
		},
		"svcPortList": func(ports []k8s.ServicePort) string {
			if len(ports) == 0 {
				return "-"
			}
			parts := make([]string, len(ports))
			for i, p := range ports {
				parts[i] = fmt.Sprintf("%d/%s", p.Port, p.Protocol)
			}
			return strings.Join(parts, ", ")
		},
		"joinStrings": strings.Join,
		"add":         func(a, b int) int { return a + b },
	}).Parse(htmlTemplate)
	if err != nil {
		return err
	}
	return t.Execute(w, data)
}

const htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>knet Network Report</title>
<style>
  :root {
    --bg: #0f172a; --bg2: #1e293b; --bg3: #334155;
    --text: #e2e8f0; --muted: #94a3b8;
    --purple: #a78bfa; --blue: #60a5fa; --green: #4ade80;
    --yellow: #fbbf24; --red: #f87171; --orange: #fb923c;
    --border: #334155;
  }
  * { box-sizing: border-box; margin: 0; padding: 0; }
  body { background: var(--bg); color: var(--text); font-family: 'Segoe UI', system-ui, sans-serif; font-size: 14px; }
  a { color: var(--purple); }
  header { background: var(--bg2); border-bottom: 1px solid var(--border); padding: 1.5rem 2rem; display: flex; align-items: center; gap: 1rem; }
  header h1 { font-size: 1.5rem; color: var(--purple); }
  header span { color: var(--muted); font-size: 0.85rem; }
  nav { background: var(--bg2); border-bottom: 1px solid var(--border); padding: 0 2rem; display: flex; gap: 0; }
  nav a { display: inline-block; padding: .75rem 1rem; color: var(--muted); text-decoration: none; border-bottom: 2px solid transparent; transition: color .15s; }
  nav a:hover { color: var(--text); }
  main { max-width: 1400px; margin: 0 auto; padding: 2rem; }
  .section { margin-bottom: 2.5rem; }
  .section h2 { font-size: 1.1rem; color: var(--purple); margin-bottom: 1rem; padding-bottom: .5rem; border-bottom: 1px solid var(--border); }
  .stats-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(150px, 1fr)); gap: 1rem; margin-bottom: 2rem; }
  .stat-card { background: var(--bg2); border: 1px solid var(--border); border-radius: 8px; padding: 1.25rem; text-align: center; }
  .stat-card .num { font-size: 2rem; font-weight: 700; }
  .stat-card .label { color: var(--muted); font-size: .8rem; margin-top: .25rem; }
  .stat-card.pods .num { color: var(--blue); }
  .stat-card.svcs .num { color: var(--green); }
  .stat-card.policies .num { color: var(--yellow); }
  .stat-card.ingresses .num { color: var(--orange); }
  table { width: 100%; border-collapse: collapse; background: var(--bg2); border-radius: 8px; overflow: hidden; }
  th { background: var(--bg3); color: var(--purple); font-weight: 600; text-align: left; padding: .6rem 1rem; font-size: .8rem; text-transform: uppercase; letter-spacing: .05em; }
  td { padding: .6rem 1rem; border-bottom: 1px solid var(--border); color: var(--text); }
  tr:last-child td { border-bottom: none; }
  tr:hover td { background: rgba(255,255,255,.02); }
  .badge { display: inline-block; padding: .15rem .5rem; border-radius: 4px; font-size: .75rem; font-weight: 600; }
  .badge.running, .badge.full { background: rgba(74,222,128,.15); color: var(--green); }
  .badge.pending, .badge.partial { background: rgba(251,191,36,.15); color: var(--yellow); }
  .badge.failed, .badge.none { background: rgba(248,113,113,.15); color: var(--red); }
  .badge.svc { background: rgba(96,165,250,.15); color: var(--blue); }
  .badge.ing { background: rgba(251,146,60,.15); color: var(--orange); }
  code, pre { font-family: 'JetBrains Mono', 'Fira Code', monospace; }
  pre.graph { background: var(--bg2); border: 1px solid var(--border); border-radius: 8px; padding: 1.25rem; overflow-x: auto; color: var(--text); font-size: .8rem; line-height: 1.5; white-space: pre; }
  .ai-box { background: var(--bg2); border-left: 4px solid var(--purple); border-radius: 0 8px 8px 0; padding: 1.25rem; white-space: pre-wrap; line-height: 1.7; }
  input[type=text] { background: var(--bg2); border: 1px solid var(--border); border-radius: 6px; color: var(--text); padding: .4rem .75rem; width: 100%; max-width: 400px; font-size: .85rem; margin-bottom: 1rem; outline: none; }
  input[type=text]:focus { border-color: var(--purple); }
  footer { text-align: center; padding: 2rem; color: var(--muted); font-size: .8rem; border-top: 1px solid var(--border); }
</style>
</head>
<body>
<header>
  <div>
    <h1>⎈ knet Network Report</h1>
    <span>Generated: {{.GeneratedAt}} | Namespace: {{if .AllNamespaces}}all{{else}}{{.Namespace}}{{end}}</span>
  </div>
</header>
<nav>
  <a href="#summary">Summary</a>
  <a href="#pods">Pods</a>
  <a href="#services">Services</a>
  <a href="#policies">Policies</a>
  <a href="#audit">Audit</a>
  {{if .ASCIIGraph}}<a href="#graph">Graph</a>{{end}}
  {{if .AIAnalysis}}<a href="#ai">AI Analysis</a>{{end}}
</nav>
<main>

<div id="summary" class="section">
  <h2>Summary</h2>
  <div class="stats-grid">
    <div class="stat-card pods"><div class="num">{{len .Pods}}</div><div class="label">Pods</div></div>
    <div class="stat-card svcs"><div class="num">{{len .Services}}</div><div class="label">Services</div></div>
    <div class="stat-card policies"><div class="num">{{len .Policies}}</div><div class="label">NetworkPolicies</div></div>
    <div class="stat-card ingresses"><div class="num">{{len .Ingresses}}</div><div class="label">Ingresses</div></div>
  </div>
</div>

<div id="pods" class="section">
  <h2>Pods ({{len .Pods}})</h2>
  <input type="text" id="pod-filter" oninput="filterTable('pod-table', this.value)" placeholder="Filter pods...">
  <table id="pod-table">
    <thead><tr><th>Namespace</th><th>Name</th><th>Status</th><th>IP</th><th>Node</th><th>Ports</th><th>Labels</th></tr></thead>
    <tbody>
    {{range .Pods}}
    <tr>
      <td>{{.Namespace}}</td>
      <td><code>{{.Name}}</code></td>
      <td><span class="badge {{.Phase | lower}}">{{.Phase}}</span></td>
      <td><code>{{.IP}}</code></td>
      <td>{{.NodeName}}</td>
      <td><code>{{portList .Ports}}</code></td>
      <td>{{labelsStr .Labels}}</td>
    </tr>
    {{end}}
    </tbody>
  </table>
</div>

<div id="services" class="section">
  <h2>Services ({{len .Services}})</h2>
  <input type="text" id="svc-filter" oninput="filterTable('svc-table', this.value)" placeholder="Filter services...">
  <table id="svc-table">
    <thead><tr><th>Namespace</th><th>Name</th><th>Type</th><th>ClusterIP</th><th>Ports</th><th>Selector</th><th>Pods</th></tr></thead>
    <tbody>
    {{range .Services}}
    <tr>
      <td>{{.Namespace}}</td>
      <td><code>{{.Name}}</code></td>
      <td><span class="badge svc">{{.Type}}</span></td>
      <td><code>{{.ClusterIP}}</code></td>
      <td><code>{{svcPortList .Ports}}</code></td>
      <td>{{labelsStr .Selector}}</td>
      <td>{{len .BackingPods}}</td>
    </tr>
    {{end}}
    </tbody>
  </table>
</div>

<div id="policies" class="section">
  <h2>NetworkPolicies ({{len .Policies}})</h2>
  <table>
    <thead><tr><th>Namespace</th><th>Name</th><th>Pod Selector</th><th>Types</th><th>Ingress Rules</th><th>Egress Rules</th></tr></thead>
    <tbody>
    {{range .Policies}}
    <tr>
      <td>{{.Namespace}}</td>
      <td><code>{{.Name}}</code></td>
      <td>{{if .SelectsAll}}<span class="badge failed">ALL pods</span>{{else}}{{labelsStr .PodSelector}}{{end}}</td>
      <td>{{joinStrings .PolicyTypes ", "}}</td>
      <td>{{len .IngressRules}}</td>
      <td>{{len .EgressRules}}</td>
    </tr>
    {{end}}
    </tbody>
  </table>
</div>

<div id="audit" class="section">
  <h2>Namespace Isolation Audit</h2>
  <table>
    <thead><tr><th>Namespace</th><th>Coverage</th><th>Policies</th><th>Has Ingress Policy</th><th>Has Egress Policy</th><th>Exposed Pods</th></tr></thead>
    <tbody>
    {{range .Audits}}
    <tr>
      <td>{{.Namespace}}</td>
      <td><span class="badge {{.CoverageLevel}}">{{.CoverageLevel}}</span></td>
      <td>{{.PolicyCount}}</td>
      <td>{{.HasIngress}}</td>
      <td>{{.HasEgress}}</td>
      <td>{{if .ExposedPods}}<span class="badge failed">{{len .ExposedPods}} exposed</span>{{else}}<span class="badge running">0</span>{{end}}</td>
    </tr>
    {{end}}
    </tbody>
  </table>
</div>

{{if .ASCIIGraph}}
<div id="graph" class="section">
  <h2>Network Graph</h2>
  <pre class="graph">{{.ASCIIGraph}}</pre>
</div>
{{end}}

{{if .MermaidGraph}}
<div class="section">
  <h2>Mermaid Diagram</h2>
  <pre class="graph">{{.MermaidGraph}}</pre>
</div>
{{end}}

{{if .AIAnalysis}}
<div id="ai" class="section">
  <h2>AI Analysis</h2>
  <div class="ai-box">{{.AIAnalysis}}</div>
</div>
{{end}}

</main>
<footer>Generated by <strong>knet</strong> — Kubernetes Network Scanner</footer>

<script>
function filterTable(tableId, query) {
  var q = query.toLowerCase();
  var rows = document.getElementById(tableId).tBodies[0].rows;
  for (var i = 0; i < rows.length; i++) {
    rows[i].style.display = rows[i].innerText.toLowerCase().includes(q) ? '' : 'none';
  }
}
// Add "lower" template function polyfill — handled server-side already
</script>
</body>
</html>`
