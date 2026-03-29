package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/themark007/knew/internal/k8s"
	"github.com/themark007/knew/internal/output"
)

var auditCmd = &cobra.Command{
	Use:   "audit",
	Short: "Audit namespace network isolation",
	Long: `Audit each namespace for NetworkPolicy coverage and isolation level.

Coverage levels:
  full     — every pod in the namespace is selected by at least one NetworkPolicy
  partial  — some pods are unprotected
  none     — no NetworkPolicies exist (all traffic allowed)

Examples:
  knet audit
  knet audit -A
  knet audit -o json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cs, defaultNS, err := k8s.NewClient(kubeconfig, kubeContext)
		if err != nil {
			return fmt.Errorf("connecting to cluster: %w", err)
		}
		ns := namespace
		if ns == "" {
			ns = defaultNS
		}

		spin := startSpinner("Auditing namespace isolation...")
		topo, err := k8s.BuildTopology(k8s.BuildOptions{
			Clientset:       cs,
			Namespace:       ns,
			AllNamespaces:   allNs,
			Timeout:         timeout,
			IncludePolicies: true,
		})
		spin.stop()
		if err != nil {
			return err
		}

		audits := k8s.AuditNamespaces(topo)

		switch outputFmt {
		case "json":
			return output.PrintJSON(os.Stdout, audits)
		case "yaml":
			return output.PrintYAML(os.Stdout, audits)
		default:
			fmt.Println()
			output.PrintAudit(os.Stdout, audits)
			fmt.Println()
			printAuditSummary(audits)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(auditCmd)
}

func printAuditSummary(audits []k8s.NamespaceAudit) {
	none, partial, full := 0, 0, 0
	for _, a := range audits {
		switch a.CoverageLevel {
		case "none":
			none++
		case "partial":
			partial++
		case "full":
			full++
		}
	}
	fmt.Printf("  Summary: %d namespaces — %d fully isolated, %d partial, %d unprotected\n",
		len(audits), full, partial, none)
	if none > 0 || partial > 0 {
		fmt.Println("  Tip: run 'knet analyze policy --ai-provider openai' for automated policy suggestions.")
	}
}
