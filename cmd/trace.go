package cmd

import (
	"fmt"
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/themark007/knew/internal/k8s"
)

var (
	traceFrom string
	traceTo   string
	tracePort int32
)

var traceCmd = &cobra.Command{
	Use:   "trace",
	Short: "Trace the network path between two pods step by step",
	Long: `Trace explains step-by-step why traffic between two pods is allowed or
blocked, walking through each relevant NetworkPolicy rule.

Unlike 'check' which just gives a verdict, 'trace' shows every decision point
in the policy evaluation — useful for debugging complex NetworkPolicy setups.

Examples:
  knet trace --from default/frontend --to default/backend --port 8080
  knet trace --from prod/api --to prod/database --port 5432`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if traceFrom == "" || traceTo == "" {
			return fmt.Errorf("--from and --to are required")
		}

		cs, defaultNS, err := k8s.NewClient(kubeconfig, kubeContext)
		if err != nil {
			return fmt.Errorf("connecting to cluster: %w", err)
		}
		ns := namespace
		if ns == "" {
			ns = defaultNS
		}

		srcRef := qualifyRef(traceFrom, ns)
		dstRef := qualifyRef(traceTo, ns)

		spin := startSpinner("Tracing network path...")
		topo, err := k8s.BuildTopology(k8s.BuildOptions{
			Clientset:       cs,
			Namespace:       ns,
			AllNamespaces:   true,
			Timeout:         timeout,
			IncludePolicies: true,
		})
		spin.stop()
		if err != nil {
			return err
		}

		result := k8s.CheckConnectivity(topo, srcRef, dstRef, tracePort)

		titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7C3AED"))
		fmt.Fprintln(os.Stdout)
		fmt.Fprintln(os.Stdout, titleStyle.Render(fmt.Sprintf("  Trace: %s → %s", srcRef, dstRef)))
		fmt.Fprintln(os.Stdout, titleStyle.Render("  ─────────────────────────────────────────"))

		okSt := lipgloss.NewStyle().Foreground(lipgloss.Color("#22C55E"))
		errSt := lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444"))
		infoSt := lipgloss.NewStyle().Foreground(lipgloss.Color("#94A3B8"))
		dimSt := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))

		for _, step := range result.TraceSteps {
			icon := "  ℹ️ "
			style := infoSt
			if step.Allowed != nil {
				if *step.Allowed {
					icon = okSt.Render("  ✓")
					style = okSt
				} else {
					icon = errSt.Render("  ✗")
					style = errSt
				}
			}
			fmt.Fprintf(os.Stdout, "%s  %s\n", icon, style.Render(fmt.Sprintf("[%d] %s", step.Step, step.Title)))
			if step.Detail != "" {
				fmt.Fprintf(os.Stdout, "       %s\n", dimSt.Render(step.Detail))
			}
		}

		fmt.Fprintln(os.Stdout)
		if result.Allowed {
			fmt.Fprintln(os.Stdout, okSt.Render("  Verdict: ALLOWED")+fmt.Sprintf("  — %s", result.Reason))
		} else {
			fmt.Fprintln(os.Stdout, errSt.Render("  Verdict: BLOCKED")+fmt.Sprintf("  — %s", result.Reason))
			fmt.Fprintln(os.Stdout)
			fmt.Fprintln(os.Stdout, dimSt.Render("  Tip: run 'knet analyze topology --ai-provider openai' for AI-guided fix suggestions"))
			os.Exit(1)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(traceCmd)
	traceCmd.Flags().StringVar(&traceFrom, "from", "", "source pod (namespace/name)")
	traceCmd.Flags().StringVar(&traceTo, "to", "", "destination pod (namespace/name)")
	traceCmd.Flags().Int32Var(&tracePort, "port", 0, "port to trace (0 = any)")
	_ = traceCmd.MarkFlagRequired("from")
	_ = traceCmd.MarkFlagRequired("to")
}
