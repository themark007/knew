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
	scanPods       bool
	scanServices   bool
	scanPolicies   bool
	scanIngress    bool
	scanExcludeSys bool
)

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan the Kubernetes network topology",
	Long: `Scan scans pods, services, NetworkPolicies and Ingresses in the target namespace(s)
and prints a summary of the network topology.

Examples:
  knet scan                        # current namespace
  knet scan -A                     # all namespaces
  knet scan -n production          # specific namespace
  knet scan -n default -o json     # JSON output
  knet scan --include-policies     # include NetworkPolicies
  knet scan -l app=nginx           # filter by label`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cs, defaultNS, err := k8s.NewClient(kubeconfig, kubeContext)
		if err != nil {
			return fmt.Errorf("connecting to cluster: %w", err)
		}
		ns := namespace
		if ns == "" {
			ns = defaultNS
		}

		spin := startSpinner("Scanning cluster...")
		topo, err := k8s.BuildTopology(k8s.BuildOptions{
			Clientset:       cs,
			Namespace:       ns,
			AllNamespaces:   allNs,
			LabelSelector:   labelSel,
			Timeout:         timeout,
			IncludePolicies: scanPolicies,
			IncludeIngress:  scanIngress,
		})
		spin.stop()
		if err != nil {
			return err
		}

		if scanExcludeSys {
			topo = excludeSystemNamespaces(topo)
		}

		switch outputFmt {
		case "json":
			return output.PrintJSON(os.Stdout, topo)
		case "yaml":
			return output.PrintYAML(os.Stdout, topo)
		default:
			wide := outputFmt == "wide"
			printScanTable(topo, wide)
		}

		if verbose {
			fmt.Printf("\nGraph stats: %s\n", graph.Build(topo).Stats())
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(scanCmd)
	scanCmd.Flags().BoolVar(&scanPods, "pods", true, "include pods in scan")
	scanCmd.Flags().BoolVar(&scanServices, "services", true, "include services in scan")
	scanCmd.Flags().BoolVar(&scanPolicies, "include-policies", false, "include NetworkPolicies in scan")
	scanCmd.Flags().BoolVar(&scanIngress, "include-ingress", false, "include Ingress resources in scan")
	scanCmd.Flags().BoolVar(&scanExcludeSys, "exclude-system", false, "exclude kube-system and other system namespaces")
}

func printScanTable(topo *k8s.Topology, wide bool) {
	scope := topo.Namespace
	if topo.AllNamespaces {
		scope = "all namespaces"
	}
	fmt.Printf("\n  ⎈ knet scan — %s  (scanned at %s)\n\n", scope, topo.ScannedAt.Format("15:04:05"))

	if len(topo.Pods) > 0 {
		fmt.Println("  PODS")
		output.PrintPods(os.Stdout, topo.Pods, wide)
		fmt.Println()
	}
	if len(topo.Services) > 0 {
		fmt.Println("  SERVICES")
		output.PrintServices(os.Stdout, topo.Services, wide)
		fmt.Println()
	}
	if len(topo.Policies) > 0 {
		fmt.Println("  NETWORK POLICIES")
		output.PrintPolicies(os.Stdout, topo.Policies)
		fmt.Println()
	}
	if len(topo.Ingresses) > 0 {
		fmt.Println("  INGRESSES")
		output.PrintIngresses(os.Stdout, topo.Ingresses)
		fmt.Println()
	}
}

func excludeSystemNamespaces(topo *k8s.Topology) *k8s.Topology {
	sys := map[string]bool{
		"kube-system": true, "kube-public": true, "kube-node-lease": true,
	}
	out := *topo
	out.Pods = nil
	for _, p := range topo.Pods {
		if !sys[p.Namespace] {
			out.Pods = append(out.Pods, p)
		}
	}
	out.Services = nil
	for _, s := range topo.Services {
		if !sys[s.Namespace] {
			out.Services = append(out.Services, s)
		}
	}
	return &out
}
