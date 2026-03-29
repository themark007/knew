package cmd

import (
	"fmt"
	"os"

	"github.com/themark007/knew/internal/graph"
	"github.com/themark007/knew/internal/k8s"
	"github.com/themark007/knew/internal/output"
	"github.com/spf13/cobra"
)

var (
	reportFile   string
	reportAIMode string
)

var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Generate a self-contained HTML network report",
	Long: `Generate a self-contained HTML report of the cluster's network topology.
The report includes:
  • Summary statistics (pods, services, policies, ingresses)
  • Filterable tables for all resource types
  • Namespace isolation audit
  • ASCII network graph
  • Mermaid diagram source
  • Optional AI analysis section (requires --ai-provider)

The HTML file has no external dependencies — works in air-gapped environments.

Examples:
  knet report --output-file report.html
  knet report -A --output-file full-report.html
  knet report --ai-provider openai --ai-key $KEY --output-file secure-report.html`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if reportFile == "" {
			return fmt.Errorf("--output-file is required")
		}

		cs, defaultNS, err := k8s.NewClient(kubeconfig, kubeContext)
		if err != nil {
			return fmt.Errorf("connecting to cluster: %w", err)
		}
		ns := namespace
		if ns == "" {
			ns = defaultNS
		}

		spin := startSpinner("Scanning cluster for report...")
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
		audits := k8s.AuditNamespaces(topo)

		// Optional AI analysis
		aiText := ""
		if aiProvider != "" {
			key := aiKey
			if key == "" {
				key = getAIKey(aiProvider)
			}
			if key != "" {
				aiText, err = runAIAnalysis(aiProvider, key, aiModel, aiBaseURL, reportAIMode, topo)
				if err != nil {
					fmt.Fprintf(os.Stderr, "  Warning: AI analysis failed: %v\n", err)
				}
			}
		}

		data := output.ReportData{
			Namespace:     ns,
			AllNamespaces: allNs,
			Pods:          topo.Pods,
			Services:      topo.Services,
			Policies:      topo.Policies,
			Ingresses:     topo.Ingresses,
			Audits:        audits,
			ASCIIGraph:    graph.RenderASCII(g, 120),
			MermaidGraph:  graph.RenderMermaid(g),
			DotGraph:      graph.RenderDOT(g),
			AIAnalysis:    aiText,
		}

		f, err := os.Create(reportFile)
		if err != nil {
			return fmt.Errorf("creating report file: %w", err)
		}
		defer f.Close()

		if err := output.RenderHTML(f, data); err != nil {
			return fmt.Errorf("rendering HTML: %w", err)
		}

		fmt.Printf("  ✓ Report written to %s\n", reportFile)
		fmt.Printf("    Resources: %d pods, %d services, %d policies, %d ingresses\n",
			len(topo.Pods), len(topo.Services), len(topo.Policies), len(topo.Ingresses))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(reportCmd)
	reportCmd.Flags().StringVar(&reportFile, "output-file", "", "output HTML file path (required)")
	reportCmd.Flags().StringVar(&aiProvider, "ai-provider", "", "AI provider for analysis section: openai|anthropic|openrouter")
	reportCmd.Flags().StringVar(&aiKey, "ai-key", "", "API key for AI provider")
	reportCmd.Flags().StringVar(&aiModel, "ai-model", "", "AI model override")
	reportCmd.Flags().StringVar(&aiBaseURL, "ai-base-url", "", "custom base URL for AI provider")
	reportCmd.Flags().StringVar(&reportAIMode, "ai-mode", "security", "AI analysis type: security|topology|policy")
	_ = reportCmd.MarkFlagRequired("output-file")
}
