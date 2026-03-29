package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/themark007/knew/internal/config"
	"github.com/themark007/knew/internal/k8s"
	"github.com/themark007/knew/internal/output"
	"github.com/spf13/cobra"
)

var (
	diffSnapshotName   string
	diffSaveSnapshotName string
)

// snapshotRecord wraps a topology with metadata for storage.
type snapshotRecord struct {
	Name      string        `json:"name"`
	SavedAt   time.Time     `json:"saved_at"`
	Namespace string        `json:"namespace"`
	Topology  *k8s.Topology `json:"topology"`
}

var diffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Compare current topology against a saved snapshot",
	Long: `Compare the current cluster topology against a previously saved snapshot.

Use 'knet diff save --name <name>' to capture the current state,
then run 'knet diff --snapshot <name>' later to see what changed.

Examples:
  knet diff save --name baseline          # save current state
  knet diff --snapshot baseline           # compare current vs baseline
  knet diff save --name pre-deploy        # before a deployment
  # ... deploy ...
  knet diff --snapshot pre-deploy         # see what changed`,
	RunE: runDiff,
}

var diffSaveCmd = &cobra.Command{
	Use:   "save",
	Short: "Save the current topology as a named snapshot",
	Long: `Save the current cluster topology to a named snapshot file.
Snapshots are stored in ~/.config/knet/snapshots/.

Examples:
  knet diff save --name baseline
  knet diff save --name pre-deploy -n production`,
	RunE: runDiffSave,
}

func init() {
	rootCmd.AddCommand(diffCmd)
	diffCmd.AddCommand(diffSaveCmd)

	diffCmd.Flags().StringVar(&diffSnapshotName, "snapshot", "", "snapshot name to compare against (required)")
	_ = diffCmd.MarkFlagRequired("snapshot")

	diffSaveCmd.Flags().StringVar(&diffSaveSnapshotName, "name", "", "name for the snapshot (required)")
	_ = diffSaveCmd.MarkFlagRequired("name")
}

func runDiffSave(cmd *cobra.Command, args []string) error {
	cs, defaultNS, err := k8s.NewClient(kubeconfig, kubeContext)
	if err != nil {
		return fmt.Errorf("connecting to cluster: %w", err)
	}
	ns := namespace
	if ns == "" {
		ns = defaultNS
	}

	spin := startSpinner("Capturing topology snapshot...")
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

	rec := snapshotRecord{
		Name:      diffSaveSnapshotName,
		SavedAt:   time.Now(),
		Namespace: ns,
		Topology:  topo,
	}
	b, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}

	dir, err := config.SnapshotsDir()
	if err != nil {
		return err
	}
	p := filepath.Join(dir, diffSaveSnapshotName+".json")
	if err := os.WriteFile(p, b, 0o600); err != nil {
		return fmt.Errorf("writing snapshot: %w", err)
	}

	fmt.Printf("  ✓ Snapshot %q saved → %s\n", diffSaveSnapshotName, p)
	fmt.Printf("    Pods: %d  Services: %d  Policies: %d  Ingresses: %d\n",
		len(topo.Pods), len(topo.Services), len(topo.Policies), len(topo.Ingresses))
	return nil
}

func runDiff(cmd *cobra.Command, args []string) error {
	dir, err := config.SnapshotsDir()
	if err != nil {
		return err
	}
	p := filepath.Join(dir, diffSnapshotName+".json")
	data, err := os.ReadFile(p)
	if err != nil {
		return fmt.Errorf("snapshot %q not found (%s): %w", diffSnapshotName, p, err)
	}
	var rec snapshotRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return fmt.Errorf("parsing snapshot: %w", err)
	}

	cs, defaultNS, err2 := k8s.NewClient(kubeconfig, kubeContext)
	if err2 != nil {
		return fmt.Errorf("connecting to cluster: %w", err2)
	}
	ns := namespace
	if ns == "" {
		ns = defaultNS
	}

	spin := startSpinner("Fetching current topology...")
	current, err := k8s.BuildTopology(k8s.BuildOptions{
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

	diff := compareTopo(rec.Topology, current)

	switch outputFmt {
	case "json":
		return output.PrintJSON(os.Stdout, diff)
	case "yaml":
		return output.PrintYAML(os.Stdout, diff)
	default:
		printDiff(rec, diff)
	}
	return nil
}

// TopoDiff holds the changes between two topology snapshots.
type TopoDiff struct {
	AddedPods      []string `json:"added_pods"`
	RemovedPods    []string `json:"removed_pods"`
	AddedServices  []string `json:"added_services"`
	RemovedServices []string `json:"removed_services"`
	AddedPolicies  []string `json:"added_policies"`
	RemovedPolicies []string `json:"removed_policies"`
	AddedIngresses []string `json:"added_ingresses"`
	RemovedIngresses []string `json:"removed_ingresses"`
}

func (d TopoDiff) HasChanges() bool {
	return len(d.AddedPods)+len(d.RemovedPods)+
		len(d.AddedServices)+len(d.RemovedServices)+
		len(d.AddedPolicies)+len(d.RemovedPolicies)+
		len(d.AddedIngresses)+len(d.RemovedIngresses) > 0
}

func compareTopo(old, current *k8s.Topology) TopoDiff {
	var d TopoDiff
	// Pods
	oldPods := podSet(old.Pods)
	newPods := podSet(current.Pods)
	for k := range newPods {
		if !oldPods[k] {
			d.AddedPods = append(d.AddedPods, k)
		}
	}
	for k := range oldPods {
		if !newPods[k] {
			d.RemovedPods = append(d.RemovedPods, k)
		}
	}
	// Services
	oldSvcs := svcSet(old.Services)
	newSvcs := svcSet(current.Services)
	for k := range newSvcs {
		if !oldSvcs[k] {
			d.AddedServices = append(d.AddedServices, k)
		}
	}
	for k := range oldSvcs {
		if !newSvcs[k] {
			d.RemovedServices = append(d.RemovedServices, k)
		}
	}
	// Policies
	oldPol := policySet(old.Policies)
	newPol := policySet(current.Policies)
	for k := range newPol {
		if !oldPol[k] {
			d.AddedPolicies = append(d.AddedPolicies, k)
		}
	}
	for k := range oldPol {
		if !newPol[k] {
			d.RemovedPolicies = append(d.RemovedPolicies, k)
		}
	}
	// Ingresses
	oldIng := ingressSet(old.Ingresses)
	newIng := ingressSet(current.Ingresses)
	for k := range newIng {
		if !oldIng[k] {
			d.AddedIngresses = append(d.AddedIngresses, k)
		}
	}
	for k := range oldIng {
		if !newIng[k] {
			d.RemovedIngresses = append(d.RemovedIngresses, k)
		}
	}
	return d
}

func printDiff(snap snapshotRecord, d TopoDiff) {
	fmt.Printf("\n  diff: current vs snapshot %q (saved %s)\n\n",
		snap.Name, snap.SavedAt.Format("2006-01-02 15:04:05"))
	if !d.HasChanges() {
		fmt.Println("  ✓  No changes detected.")
		return
	}
	printDiffSection("Pods", d.AddedPods, d.RemovedPods)
	printDiffSection("Services", d.AddedServices, d.RemovedServices)
	printDiffSection("NetworkPolicies", d.AddedPolicies, d.RemovedPolicies)
	printDiffSection("Ingresses", d.AddedIngresses, d.RemovedIngresses)
}

func printDiffSection(title string, added, removed []string) {
	if len(added)+len(removed) == 0 {
		return
	}
	fmt.Printf("  %s:\n", title)
	for _, r := range removed {
		fmt.Printf("    \033[31m- %s\033[0m\n", r)
	}
	for _, a := range added {
		fmt.Printf("    \033[32m+ %s\033[0m\n", a)
	}
}

func podSet(pods []k8s.PodInfo) map[string]bool {
	m := make(map[string]bool)
	for _, p := range pods {
		m[p.Namespace+"/"+p.Name] = true
	}
	return m
}

func svcSet(svcs []k8s.ServiceInfo) map[string]bool {
	m := make(map[string]bool)
	for _, s := range svcs {
		m[s.Namespace+"/"+s.Name] = true
	}
	return m
}

func policySet(pols []k8s.NetworkPolicyInfo) map[string]bool {
	m := make(map[string]bool)
	for _, p := range pols {
		m[p.Namespace+"/"+p.Name] = true
	}
	return m
}

func ingressSet(ings []k8s.IngressInfo) map[string]bool {
	m := make(map[string]bool)
	for _, i := range ings {
		m[i.Namespace+"/"+i.Name] = true
	}
	return m
}
