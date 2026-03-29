package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/themark007/knew/internal/k8s"
	"github.com/themark007/knew/internal/output"
)

var podsCmd = &cobra.Command{
	Use:   "pods",
	Short: "List pods with network details",
	Long: `List pods in the target namespace(s) with their IPs, ports, labels and status.

Examples:
  knet pods                    # current namespace
  knet pods -A                 # all namespaces
  knet pods -n production -o wide
  knet pods -l app=frontend`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cs, defaultNS, err := k8s.NewClient(kubeconfig, kubeContext)
		if err != nil {
			return fmt.Errorf("connecting to cluster: %w", err)
		}
		ns := namespace
		if ns == "" {
			ns = defaultNS
		}

		spin := startSpinner("Fetching pods...")
		topo, err := k8s.BuildTopology(k8s.BuildOptions{
			Clientset:     cs,
			Namespace:     ns,
			AllNamespaces: allNs,
			LabelSelector: labelSel,
			Timeout:       timeout,
		})
		spin.stop()
		if err != nil {
			return err
		}

		switch outputFmt {
		case "json":
			return output.PrintJSON(os.Stdout, topo.Pods)
		case "yaml":
			return output.PrintYAML(os.Stdout, topo.Pods)
		default:
			output.PrintPods(os.Stdout, topo.Pods, outputFmt == "wide")
		}
		fmt.Printf("\n  Total: %d pod(s)\n", len(topo.Pods))
		return nil
	},
}

var servicesCmd = &cobra.Command{
	Use:     "services",
	Aliases: []string{"svc"},
	Short:   "List services with endpoint details",
	Long: `List services in the target namespace(s) with their port mappings,
selectors, and the number of backing pods resolved via Endpoints.

Examples:
  knet services
  knet services -n staging -o wide
  knet svc -A`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cs, defaultNS, err := k8s.NewClient(kubeconfig, kubeContext)
		if err != nil {
			return fmt.Errorf("connecting to cluster: %w", err)
		}
		ns := namespace
		if ns == "" {
			ns = defaultNS
		}

		spin := startSpinner("Fetching services...")
		topo, err := k8s.BuildTopology(k8s.BuildOptions{
			Clientset:     cs,
			Namespace:     ns,
			AllNamespaces: allNs,
			Timeout:       timeout,
		})
		spin.stop()
		if err != nil {
			return err
		}

		switch outputFmt {
		case "json":
			return output.PrintJSON(os.Stdout, topo.Services)
		case "yaml":
			return output.PrintYAML(os.Stdout, topo.Services)
		default:
			output.PrintServices(os.Stdout, topo.Services, outputFmt == "wide")
		}
		fmt.Printf("\n  Total: %d service(s)\n", len(topo.Services))
		return nil
	},
}

var policiesCmd = &cobra.Command{
	Use:     "policies",
	Aliases: []string{"netpol"},
	Short:   "List NetworkPolicies",
	Long: `List NetworkPolicies in the target namespace(s) showing pod selectors
and ingress/egress rule counts.

Examples:
  knet policies
  knet policies -A
  knet netpol -n production -o yaml`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cs, defaultNS, err := k8s.NewClient(kubeconfig, kubeContext)
		if err != nil {
			return fmt.Errorf("connecting to cluster: %w", err)
		}
		ns := namespace
		if ns == "" {
			ns = defaultNS
		}

		spin := startSpinner("Fetching NetworkPolicies...")
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

		switch outputFmt {
		case "json":
			return output.PrintJSON(os.Stdout, topo.Policies)
		case "yaml":
			return output.PrintYAML(os.Stdout, topo.Policies)
		default:
			output.PrintPolicies(os.Stdout, topo.Policies)
		}
		fmt.Printf("\n  Total: %d NetworkPolicy/ies\n", len(topo.Policies))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(podsCmd)
	rootCmd.AddCommand(servicesCmd)
	rootCmd.AddCommand(policiesCmd)
}
