package ai

import (
	"fmt"
	"strings"

	"github.com/themark007/knew/internal/k8s"
)

// SecurityAuditPrompt builds a prompt asking the AI to audit the network for security issues.
func SecurityAuditPrompt(topo *k8s.Topology) string {
	return fmt.Sprintf(`You are auditing a Kubernetes cluster's network security.

Cluster snapshot:
%s

Please:
1. Identify any namespaces that lack NetworkPolicies (fully open to all traffic).
2. Highlight services exposed without proper ingress restrictions.
3. Note any pods running without resource labels (hard to target with policies).
4. Flag any NetworkPolicy rules that are overly permissive (e.g., allow all namespaces).
5. Provide a risk rating: LOW / MEDIUM / HIGH / CRITICAL.
6. Give 3–5 specific, actionable recommendations.

Be concise and focus on the most impactful findings.`, topologySummary(topo))
}

// TopologyExplainPrompt builds a prompt asking for a plain-English topology explanation.
func TopologyExplainPrompt(topo *k8s.Topology) string {
	return fmt.Sprintf(`Explain the following Kubernetes network topology in plain language.
Describe how traffic flows from external users to backend pods, which services are exposed,
and the overall architecture pattern (e.g., microservices, monolith, sidecar mesh).

Cluster snapshot:
%s

Keep the explanation accessible to a developer who is not a Kubernetes expert.`, topologySummary(topo))
}

// PolicySuggestPrompt asks the AI to suggest NetworkPolicy improvements.
func PolicySuggestPrompt(topo *k8s.Topology) string {
	return fmt.Sprintf(`Review the NetworkPolicies in this Kubernetes cluster and suggest improvements.

Cluster snapshot:
%s

For each namespace:
1. Identify what is currently missing (no egress policy, no default-deny, etc.).
2. Suggest specific NetworkPolicy YAML snippets that would improve isolation.
3. Explain the trade-offs of each suggestion.

Focus on least-privilege access principles.`, topologySummary(topo))
}

// GeneratePolicyPrompt asks the AI to generate a NetworkPolicy from a description.
func GeneratePolicyPrompt(description string, topo *k8s.Topology) string {
	return fmt.Sprintf(`Generate a Kubernetes NetworkPolicy YAML based on this requirement:
"%s"

Available pods and services in the cluster:
%s

Output ONLY valid Kubernetes YAML. Include comments explaining each rule.
Follow these best practices:
- Use specific pod/namespace selectors rather than wildcards where possible
- Include both ingress and egress rules where relevant
- Add a default-deny rule if it makes sense for the use case`, description, topologySummary(topo))
}

// TraceExplainPrompt asks the AI to explain a connectivity trace result.
func TraceExplainPrompt(srcRef, dstRef string, result interface{}) string {
	return fmt.Sprintf(`Explain in plain language why traffic %s→%s is allowed or blocked in this Kubernetes cluster.
Trace result:
%v

Please:
1. Summarize the verdict (allowed / blocked).
2. Explain which NetworkPolicy rules are involved.
3. If blocked, explain exactly what change would allow the traffic.
4. If allowed, explain whether this is intentional and secure.`, srcRef, dstRef, result)
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func topologySummary(topo *k8s.Topology) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Scanned at: %s\n", topo.ScannedAt.Format("2006-01-02 15:04:05")))
	b.WriteString(fmt.Sprintf("Namespaces: %s\n\n", nsScope(topo)))

	b.WriteString(fmt.Sprintf("Pods (%d):\n", len(topo.Pods)))
	for _, p := range topo.Pods {
		labels := labelsStr(p.Labels)
		b.WriteString(fmt.Sprintf("  %s/%s  IP=%s  phase=%s  labels={%s}\n",
			p.Namespace, p.Name, p.IP, p.Phase, labels))
	}

	b.WriteString(fmt.Sprintf("\nServices (%d):\n", len(topo.Services)))
	for _, s := range topo.Services {
		ports := make([]string, len(s.Ports))
		for i, p := range s.Ports {
			ports[i] = fmt.Sprintf("%d", p.Port)
		}
		b.WriteString(fmt.Sprintf("  %s/%s  type=%s  clusterIP=%s  ports=%s  selector={%s}  pods=%v\n",
			s.Namespace, s.Name, s.Type, s.ClusterIP, strings.Join(ports, ","), labelsStr(s.Selector), s.BackingPods))
	}

	b.WriteString(fmt.Sprintf("\nNetworkPolicies (%d):\n", len(topo.Policies)))
	for _, np := range topo.Policies {
		b.WriteString(fmt.Sprintf("  %s/%s  types=%v  selector={%s}  ingress_rules=%d  egress_rules=%d\n",
			np.Namespace, np.Name, np.PolicyTypes, labelsStr(np.PodSelector),
			len(np.IngressRules), len(np.EgressRules)))
	}

	b.WriteString(fmt.Sprintf("\nIngresses (%d):\n", len(topo.Ingresses)))
	for _, ing := range topo.Ingresses {
		b.WriteString(fmt.Sprintf("  %s/%s  tls=%v  rules=%d\n",
			ing.Namespace, ing.Name, ing.TLS, len(ing.Rules)))
	}

	return b.String()
}

func nsScope(topo *k8s.Topology) string {
	if topo.AllNamespaces {
		return "all"
	}
	return topo.Namespace
}

func labelsStr(m map[string]string) string {
	if len(m) == 0 {
		return ""
	}
	var parts []string
	for k, v := range m {
		parts = append(parts, k+"="+v)
	}
	return strings.Join(parts, ", ")
}
