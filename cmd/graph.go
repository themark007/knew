package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/themark007/knew/internal/graph"
	"github.com/themark007/knew/internal/k8s"
)

var (
	graphFormat  string
	graphGroupBy string
	graphWidth   int
)

var graphCmd = &cobra.Command{
	Use:   "graph [pods|services|full]",
	Short: "Visualize the network topology as a graph",
	Long: `Render the Kubernetes network topology as a graph in various formats.

Subcommands:
  pods      — show only pod connectivity
  services  — show services and their backing pods
  full      — show full topology (default)

Formats (--format):
  ascii     — ASCII art in terminal (default)
  tui       — interactive bubbletea TUI (arrow keys to navigate)
  dot       — Graphviz DOT format (pipe to dot -Tpng > graph.png)
  mermaid   — Mermaid diagram text

Examples:
  knet graph                           # full ASCII art
  knet graph full --format tui         # interactive explorer
  knet graph full --format dot | dot -Tsvg > net.svg
  knet graph full --format mermaid     # for Markdown embedding
  knet graph -A --format ascii         # all namespaces`,
	ValidArgs: []string{"pods", "services", "full"},
	RunE:      runGraph,
}

func init() {
	rootCmd.AddCommand(graphCmd)
	graphCmd.Flags().StringVar(&graphFormat, "format", "ascii", "output format: ascii|tui|dot|mermaid")
	graphCmd.Flags().StringVar(&graphGroupBy, "group-by", "namespace", "group nodes by: namespace|label")
	graphCmd.Flags().IntVar(&graphWidth, "width", 0, "graph width in columns (0=auto)")
}

func runGraph(cmd *cobra.Command, args []string) error {
	mode := "full"
	if len(args) > 0 {
		mode = args[0]
	}

	cs, defaultNS, err := k8s.NewClient(kubeconfig, kubeContext)
	if err != nil {
		return fmt.Errorf("connecting to cluster: %w", err)
	}
	ns := namespace
	if ns == "" {
		ns = defaultNS
	}

	spin := startSpinner("Building graph...")
	topo, err := k8s.BuildTopology(k8s.BuildOptions{
		Clientset:       cs,
		Namespace:       ns,
		AllNamespaces:   allNs,
		LabelSelector:   labelSel,
		Timeout:         timeout,
		IncludePolicies: true,
		IncludeIngress:  true,
	})
	spin.stop()
	if err != nil {
		return err
	}

	g := graph.Build(topo)

	// Filter graph by mode
	if mode == "pods" {
		g = filterGraphNodes(g, graph.NodePod)
	} else if mode == "services" {
		g = filterGraphNodes(g, graph.NodeService, graph.NodePod)
	}

	switch graphFormat {
	case "tui":
		return graph.RunTUI(g)
	case "dot":
		fmt.Print(graph.RenderDOT(g))
	case "mermaid":
		fmt.Print(graph.RenderMermaid(g))
	default: // ascii
		fmt.Print(graph.RenderASCII(g, graphWidth))
	}
	return nil
}

// filterGraphNodes returns a new graph containing only nodes of the given types (and edges between them).
func filterGraphNodes(g *graph.Graph, types ...graph.NodeType) *graph.Graph {
	allowed := make(map[graph.NodeType]bool)
	for _, t := range types {
		allowed[t] = true
	}
	allowedIDs := make(map[string]bool)
	for _, n := range g.Nodes {
		if allowed[n.Type] {
			allowedIDs[n.ID] = true
		}
	}
	ng := &graph.Graph{}
	for _, n := range g.Nodes {
		if allowedIDs[n.ID] {
			ng.Nodes = append(ng.Nodes, n)
		}
	}
	for _, e := range g.Edges {
		if allowedIDs[e.From] && allowedIDs[e.To] {
			ng.Edges = append(ng.Edges, e)
		}
	}
	return ng
}

// startSpinner returns a simple terminal spinner.
// It writes to stderr so it doesn't pollute structured output.
func startSpinner(msg string) *simpleSpinner {
	s := &simpleSpinner{msg: msg, done: make(chan struct{})}
	if outputFmt == "json" || outputFmt == "yaml" {
		return s // no spinner for structured output
	}
	go s.run()
	return s
}

type simpleSpinner struct {
	msg  string
	done chan struct{}
}

func (s *simpleSpinner) run() {
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	i := 0
	for {
		select {
		case <-s.done:
			fmt.Fprintf(os.Stderr, "\r\033[K")
			return
		default:
			fmt.Fprintf(os.Stderr, "\r%s %s", frames[i%len(frames)], s.msg)
			i++
			// small busy wait — good enough for a spinner
			for j := 0; j < 5000000; j++ {
			}
		}
	}
}

func (s *simpleSpinner) stop() {
	select {
	case <-s.done:
	default:
		close(s.done)
	}
}
