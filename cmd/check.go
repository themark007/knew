package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/themark007/knew/internal/k8s"
	"github.com/themark007/knew/internal/output"
)

var (
	checkFrom string
	checkTo   string
	checkPort int32
)

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Check if a pod can reach another pod or service",
	Long: `Perform a static NetworkPolicy analysis to determine whether traffic
is allowed from a source pod to a destination pod or service.

This is a static analysis (no live traffic) based on NetworkPolicy rules.

Examples:
  knet check --from default/frontend --to default/backend --port 8080
  knet check --from frontend --to backend             # auto-namespace lookup
  knet check --from mynamespace/app --to db --port 5432 -o json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if checkFrom == "" || checkTo == "" {
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

		// Qualify refs with namespace if not already qualified
		srcRef := qualifyRef(checkFrom, ns)
		dstRef := qualifyRef(checkTo, ns)

		spin := startSpinner("Fetching topology for connectivity analysis...")
		topo, err := k8s.BuildTopology(k8s.BuildOptions{
			Clientset:       cs,
			Namespace:       ns,
			AllNamespaces:   true, // connectivity may cross namespaces
			Timeout:         timeout,
			IncludePolicies: true,
		})
		spin.stop()
		if err != nil {
			return err
		}

		result := k8s.CheckConnectivity(topo, srcRef, dstRef, checkPort)

		switch outputFmt {
		case "json":
			return output.PrintJSON(os.Stdout, result)
		case "yaml":
			return output.PrintYAML(os.Stdout, result)
		default:
			output.PrintConnectivity(os.Stdout, srcRef, dstRef, checkPort, result)
		}

		if !result.Allowed {
			os.Exit(1)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(checkCmd)
	checkCmd.Flags().StringVar(&checkFrom, "from", "", "source pod (namespace/name or name)")
	checkCmd.Flags().StringVar(&checkTo, "to", "", "destination pod or service (namespace/name or name)")
	checkCmd.Flags().Int32Var(&checkPort, "port", 0, "destination port to check (0 = any port)")
	_ = checkCmd.MarkFlagRequired("from")
	_ = checkCmd.MarkFlagRequired("to")
}

func qualifyRef(ref, defaultNS string) string {
	if strings.Contains(ref, "/") {
		return ref
	}
	return defaultNS + "/" + ref
}
