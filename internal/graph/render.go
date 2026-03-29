package graph

import (
	"fmt"
	"strings"
)

// RenderASCII produces a human-readable ASCII art graph in the terminal.
// width is the max column width (0 = auto 100).
func RenderASCII(g *Graph, width int) string {
	if width <= 0 {
		width = 100
	}

	var b strings.Builder

	// Separate nodes by type for layered rendering
	var ingresses, services, pods []Node
	for _, n := range g.Nodes {
		switch n.Type {
		case NodeIngress:
			ingresses = append(ingresses, n)
		case NodeService:
			services = append(services, n)
		default:
			pods = append(pods, n)
		}
	}

	writeSep := func(label string) {
		line := "── " + label + " " + strings.Repeat("─", max(0, width-len(label)-4))
		b.WriteString(line + "\n")
	}

	renderLayer := func(nodes []Node, icon string) {
		if len(nodes) == 0 {
			return
		}
		for _, n := range nodes {
			box := renderBox(n, icon, width)
			b.WriteString(box)
			// Print outgoing edges from this node
			for _, e := range g.OutEdgesFrom(n.ID) {
				target := g.NodeByID(e.To)
				if target == nil {
					continue
				}
				arrow := edgeArrow(e.Type)
				label := ""
				if e.Label != "" {
					label = fmt.Sprintf(" [%s]", truncate(e.Label, 30))
				}
				b.WriteString(fmt.Sprintf("  %s %s %s/%s%s\n", arrow, nodeIcon(target.Type), target.Namespace, target.Name, label))
			}
		}
	}

	writeSep("INGRESSES")
	renderLayer(ingresses, "⮕ ")
	writeSep("SERVICES")
	renderLayer(services, "⚙ ")
	writeSep("PODS")
	renderLayer(pods, "● ")
	b.WriteString(strings.Repeat("─", width) + "\n")
	b.WriteString(fmt.Sprintf("  %s\n", g.Stats()))

	return b.String()
}

func renderBox(n Node, icon string, width int) string {
	topLine := "┌" + strings.Repeat("─", width-2) + "┐"
	botLine := "└" + strings.Repeat("─", width-2) + "┘"
	title := fmt.Sprintf(" %s%s/%s", icon, n.Namespace, n.Name)
	extra := ""
	if n.Extra != "" {
		extra = fmt.Sprintf("   %s", truncate(n.Extra, width-5))
	}
	return topLine + "\n" +
		"│" + pad(title, width-2) + "│\n" +
		"│" + pad(extra, width-2) + "│\n" +
		botLine + "\n"
}

func edgeArrow(t EdgeType) string {
	switch t {
	case EdgeEndpoint:
		return "  ├─endpoint──►"
	case EdgeSelector:
		return "  ├─selector──►"
	case EdgeIngressRoute:
		return "  ├─route─────►"
	case EdgePolicyAllow:
		return "  ├─allow─────►"
	default:
		return "  ├──────────►"
	}
}

func nodeIcon(t NodeType) string {
	switch t {
	case NodePod:
		return "[POD] "
	case NodeService:
		return "[SVC] "
	case NodeIngress:
		return "[ING] "
	default:
		return "[EXT] "
	}
}

func pad(s string, width int) string {
	if len(s) >= width {
		return s[:width]
	}
	return s + strings.Repeat(" ", width-len(s))
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}

// RenderDOT emits a Graphviz DOT representation of the graph.
func RenderDOT(g *Graph) string {
	var b strings.Builder
	b.WriteString("digraph knet {\n")
	b.WriteString("  rankdir=LR;\n")
	b.WriteString("  node [shape=box fontname=\"Helvetica\"];\n\n")

	for _, n := range g.Nodes {
		shape := "box"
		color := "#aaaaaa"
		switch n.Type {
		case NodePod:
			shape = "ellipse"
			color = "#4fc3f7"
		case NodeService:
			shape = "box"
			color = "#81c784"
		case NodeIngress:
			shape = "diamond"
			color = "#ffb74d"
		}
		label := fmt.Sprintf("%s\\n%s/%s", strings.ToUpper(string(n.Type)), n.Namespace, n.Name)
		b.WriteString(fmt.Sprintf("  %q [label=%q shape=%s style=filled fillcolor=%q];\n",
			n.ID, label, shape, color))
	}
	b.WriteString("\n")
	for _, e := range g.Edges {
		style := "solid"
		if e.Type == EdgePolicyAllow {
			style = "dashed"
		}
		b.WriteString(fmt.Sprintf("  %q -> %q [label=%q style=%s];\n", e.From, e.To, e.Label, style))
	}
	b.WriteString("}\n")
	return b.String()
}

// RenderMermaid emits a Mermaid diagram representation.
func RenderMermaid(g *Graph) string {
	var b strings.Builder
	b.WriteString("graph LR\n")

	clean := func(s string) string {
		return strings.NewReplacer("/", "_", ":", "_", "-", "_", ".", "_").Replace(s)
	}

	for _, n := range g.Nodes {
		id := clean(n.ID)
		label := fmt.Sprintf("%s/%s", n.Namespace, n.Name)
		switch n.Type {
		case NodePod:
			b.WriteString(fmt.Sprintf("  %s((%s))\n", id, label))
		case NodeService:
			b.WriteString(fmt.Sprintf("  %s[%s]\n", id, label))
		case NodeIngress:
			b.WriteString(fmt.Sprintf("  %s{%s}\n", id, label))
		default:
			b.WriteString(fmt.Sprintf("  %s[%s]\n", id, label))
		}
	}

	for _, e := range g.Edges {
		from := clean(e.From)
		to := clean(e.To)
		label := e.Label
		if label == "" {
			label = string(e.Type)
		}
		b.WriteString(fmt.Sprintf("  %s -->|%s| %s\n", from, label, to))
	}
	return b.String()
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
