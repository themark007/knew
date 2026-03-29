// Package output renders topology data in multiple formats.
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/themark007/knew/internal/k8s"
	"sigs.k8s.io/yaml"
)

// ─── Styles ───────────────────────────────────────────────────────────────────

var (
	headerStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7C3AED"))
	okStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#22C55E"))
	warnStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B"))
	dangerStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444"))
	dimStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	podColor     = lipgloss.NewStyle().Foreground(lipgloss.Color("#4FC3F7"))
	svcColor     = lipgloss.NewStyle().Foreground(lipgloss.Color("#81C784"))
	ingColor     = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFB74D"))
)

// ─── Table renderer ───────────────────────────────────────────────────────────

// PrintPods renders pods in table format.
func PrintPods(w io.Writer, pods []k8s.PodInfo, wide bool) {
	tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
	if wide {
		fmt.Fprintln(tw, headerStyle.Render("NAMESPACE\tNAME\tSTATUS\tIP\tNODE\tPORTS\tAGE"))
	} else {
		fmt.Fprintln(tw, headerStyle.Render("NAMESPACE\tNAME\tSTATUS\tIP\tAGE"))
	}
	for _, p := range pods {
		status := statusStyle(p.Phase, p.Ready)
		ports := portList(p.Ports)
		age := humanAge(p.CreatedAt)
		if wide {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				p.Namespace, podColor.Render(p.Name), status, p.IP, p.NodeName, ports, age)
		} else {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
				p.Namespace, podColor.Render(p.Name), status, p.IP, age)
		}
	}
	tw.Flush()
}

// PrintServices renders services in table format.
func PrintServices(w io.Writer, svcs []k8s.ServiceInfo, wide bool) {
	tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
	if wide {
		fmt.Fprintln(tw, headerStyle.Render("NAMESPACE\tNAME\tTYPE\tCLUSTER-IP\tEXTERNAL-IP\tPORTS\tBACKING-PODS\tAGE"))
	} else {
		fmt.Fprintln(tw, headerStyle.Render("NAMESPACE\tNAME\tTYPE\tCLUSTER-IP\tPORTS\tAGE"))
	}
	for _, s := range svcs {
		ports := svcPortList(s.Ports)
		ext := s.ExternalIP
		if ext == "" {
			ext = dimStyle.Render("<none>")
		}
		age := humanAge(s.CreatedAt)
		backing := fmt.Sprintf("%d pods", len(s.BackingPods))
		if wide {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				s.Namespace, svcColor.Render(s.Name), s.Type, s.ClusterIP, ext, ports, backing, age)
		} else {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
				s.Namespace, svcColor.Render(s.Name), s.Type, s.ClusterIP, ports, age)
		}
	}
	tw.Flush()
}

// PrintPolicies renders NetworkPolicies in table format.
func PrintPolicies(w io.Writer, policies []k8s.NetworkPolicyInfo) {
	tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
	fmt.Fprintln(tw, headerStyle.Render("NAMESPACE\tNAME\tPOD-SELECTOR\tTYPES\tINGRESS-RULES\tEGRESS-RULES"))
	for _, np := range policies {
		sel := labelsStr(np.PodSelector)
		if np.SelectsAll {
			sel = warnStyle.Render("ALL pods")
		}
		types := strings.Join(np.PolicyTypes, ",")
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%d\t%d\n",
			np.Namespace, ingColor.Render(np.Name), sel, types,
			len(np.IngressRules), len(np.EgressRules))
	}
	tw.Flush()
}

// PrintIngresses renders Ingresses in table format.
func PrintIngresses(w io.Writer, ingresses []k8s.IngressInfo) {
	tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
	fmt.Fprintln(tw, headerStyle.Render("NAMESPACE\tNAME\tTLS\tRULES\tAGE"))
	for _, ing := range ingresses {
		tls := dimStyle.Render("false")
		if ing.TLS {
			tls = okStyle.Render("true")
		}
		age := humanAge(ing.CreatedAt)
		fmt.Fprintf(tw, "%s\t%s\t%s\t%d\t%s\n",
			ing.Namespace, ingColor.Render(ing.Name), tls, len(ing.Rules), age)
	}
	tw.Flush()
}

// PrintAudit renders namespace audit results.
func PrintAudit(w io.Writer, audits []k8s.NamespaceAudit) {
	tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
	fmt.Fprintln(tw, headerStyle.Render("NAMESPACE\tCOVERAGE\tPOLICIES\tHAS-INGRESS\tHAS-EGRESS\tEXPOSED-PODS"))
	for _, a := range audits {
		level := coverageStyle(a.CoverageLevel)
		exposed := fmt.Sprintf("%d", len(a.ExposedPods))
		if len(a.ExposedPods) > 0 {
			exposed = dangerStyle.Render(exposed)
		}
		fmt.Fprintf(tw, "%s\t%s\t%d\t%v\t%v\t%s\n",
			a.Namespace, level, a.PolicyCount, a.HasIngress, a.HasEgress, exposed)
	}
	tw.Flush()
}

// PrintConnectivity renders a connectivity check result.
func PrintConnectivity(w io.Writer, src, dst string, port int32, res k8s.ConnectivityResult) {
	verdict := dangerStyle.Render("✗ BLOCKED")
	if res.Allowed {
		verdict = okStyle.Render("✓ ALLOWED")
	}
	fmt.Fprintf(w, "\n  %s → %s", src, dst)
	if port > 0 {
		fmt.Fprintf(w, ":%d", port)
	}
	fmt.Fprintf(w, "  %s\n\n", verdict)
	fmt.Fprintf(w, "  Reason: %s\n\n", res.Reason)
	if len(res.TraceSteps) > 0 {
		fmt.Fprintln(w, "  Trace:")
		for _, step := range res.TraceSteps {
			icon := "  ℹ"
			if step.Allowed != nil {
				if *step.Allowed {
					icon = okStyle.Render("  ✓")
				} else {
					icon = dangerStyle.Render("  ✗")
				}
			}
			fmt.Fprintf(w, "  %s [%d] %s\n", icon, step.Step, step.Title)
			if step.Detail != "" {
				fmt.Fprintf(w, "         %s\n", dimStyle.Render(step.Detail))
			}
		}
		fmt.Fprintln(w)
	}
	if len(res.MatchedIngress) > 0 {
		fmt.Fprintf(w, "  Matched ingress policies: %s\n", strings.Join(res.MatchedIngress, ", "))
	}
	if len(res.MatchedEgress) > 0 {
		fmt.Fprintf(w, "  Matched egress policies:  %s\n", strings.Join(res.MatchedEgress, ", "))
	}
}

// ─── JSON / YAML ─────────────────────────────────────────────────────────────

// PrintJSON marshals v to pretty-printed JSON.
func PrintJSON(w io.Writer, v interface{}) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// PrintYAML marshals v to YAML.
func PrintYAML(w io.Writer, v interface{}) error {
	b, err := yaml.Marshal(v)
	if err != nil {
		return err
	}
	_, err = w.Write(b)
	return err
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func statusStyle(phase string, ready bool) string {
	switch phase {
	case "Running":
		if ready {
			return okStyle.Render(phase)
		}
		return warnStyle.Render(phase)
	case "Pending":
		return warnStyle.Render(phase)
	case "Failed":
		return dangerStyle.Render(phase)
	default:
		return dimStyle.Render(phase)
	}
}

func coverageStyle(level string) string {
	switch level {
	case "full":
		return okStyle.Render("full")
	case "partial":
		return warnStyle.Render("partial")
	default:
		return dangerStyle.Render("none")
	}
}

func portList(ports []k8s.PortInfo) string {
	if len(ports) == 0 {
		return dimStyle.Render("<none>")
	}
	parts := make([]string, len(ports))
	for i, p := range ports {
		parts[i] = fmt.Sprintf("%d/%s", p.Port, p.Protocol)
	}
	return strings.Join(parts, ",")
}

func svcPortList(ports []k8s.ServicePort) string {
	if len(ports) == 0 {
		return dimStyle.Render("<none>")
	}
	parts := make([]string, len(ports))
	for i, p := range ports {
		if p.NodePort > 0 {
			parts[i] = fmt.Sprintf("%d:%d/%s", p.Port, p.NodePort, p.Protocol)
		} else {
			parts[i] = fmt.Sprintf("%d/%s", p.Port, p.Protocol)
		}
	}
	return strings.Join(parts, ",")
}

func humanAge(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

func labelsStr(m map[string]string) string {
	if len(m) == 0 {
		return dimStyle.Render("<none>")
	}
	var parts []string
	for k, v := range m {
		parts = append(parts, k+"="+v)
	}
	return strings.Join(parts, ", ")
}
