package k8s

import (
	"context"
	"flag"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

// ─── Core types ──────────────────────────────────────────────────────────────

// PodInfo holds discovered pod metadata.
type PodInfo struct {
	Name       string
	Namespace  string
	IP         string
	NodeName   string
	Labels     map[string]string
	Ports      []PortInfo
	Phase      string
	Ready      bool
	Conditions []string
	CreatedAt  time.Time
}

// PortInfo describes a container port.
type PortInfo struct {
	Name     string
	Port     int32
	Protocol string
}

// ServicePort describes a service port mapping.
type ServicePort struct {
	Name       string
	Port       int32
	TargetPort string
	Protocol   string
	NodePort   int32
}

// ServiceInfo holds discovered service metadata.
type ServiceInfo struct {
	Name        string
	Namespace   string
	ClusterIP   string
	ExternalIP  string
	Type        string
	Ports       []ServicePort
	Selector    map[string]string
	BackingPods []string
	CreatedAt   time.Time
}

// NetworkPolicyRule represents a simplified ingress or egress rule.
type NetworkPolicyRule struct {
	Ports              []int32
	FromNamespaces     []string // namespace names or label selectors
	FromPodSelectors   []map[string]string
	ToNamespaces       []string
	ToPodSelectors     []map[string]string
	AllowAllNamespaces bool
	AllowAllPods       bool
	IPBlocks           []string
}

// NetworkPolicyInfo holds a parsed NetworkPolicy.
type NetworkPolicyInfo struct {
	Name         string
	Namespace    string
	PodSelector  map[string]string
	PolicyTypes  []string // "Ingress", "Egress"
	IngressRules []NetworkPolicyRule
	EgressRules  []NetworkPolicyRule
	SelectsAll   bool // podSelector: {} — applies to all pods in namespace
}

// IngressRule describes a single path rule inside an Ingress.
type IngressRule struct {
	Host        string
	Path        string
	PathType    string
	ServiceName string
	ServicePort int32
}

// IngressInfo holds discovered Ingress resource metadata.
type IngressInfo struct {
	Name      string
	Namespace string
	Rules     []IngressRule
	TLS       bool
	TLSHOSTS  []string
	CreatedAt time.Time
}

// Topology is the central data structure — all discovered network resources.
type Topology struct {
	Namespace     string
	AllNamespaces bool
	ScannedAt     time.Time
	Pods          []PodInfo
	Services      []ServiceInfo
	Policies      []NetworkPolicyInfo
	Ingresses     []IngressInfo
}

// ─── Client ──────────────────────────────────────────────────────────────────

// NewClient creates a Kubernetes clientset from the given kubeconfig and context.
func NewClient(kubeconfigPath, kubeContext string) (*kubernetes.Clientset, string, error) {
	// Silence deprecation warnings from the kubernetes API
	klog.InitFlags(nil)
	_ = flag.Set("logtostderr", "false")

	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if kubeconfigPath != "" {
		loadingRules.ExplicitPath = kubeconfigPath
	}
	overrides := &clientcmd.ConfigOverrides{}
	if kubeContext != "" {
		overrides.CurrentContext = kubeContext
	}
	cc := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides)
	ns, _, err := cc.Namespace()
	if err != nil || ns == "" {
		ns = "default"
	}
	cfg, err := cc.ClientConfig()
	if err != nil {
		return nil, ns, fmt.Errorf("cannot build kubeconfig: %w", err)
	}
	cs, err := kubernetes.NewForConfig(cfg)
	return cs, ns, err
}

// ─── Build Topology ──────────────────────────────────────────────────────────

// BuildOptions controls what the topology scan includes.
type BuildOptions struct {
	Clientset       *kubernetes.Clientset
	Namespace       string
	AllNamespaces   bool
	LabelSelector   string
	Timeout         int
	IncludePolicies bool
	IncludeIngress  bool
}

// BuildTopology discovers all relevant Kubernetes resources and assembles a Topology.
func BuildTopology(opts BuildOptions) (*Topology, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(opts.Timeout)*time.Second)
	defer cancel()

	topo := &Topology{
		Namespace:     opts.Namespace,
		AllNamespaces: opts.AllNamespaces,
		ScannedAt:     time.Now(),
	}

	ns := opts.Namespace
	if opts.AllNamespaces {
		ns = ""
	}

	listOpts := metav1.ListOptions{LabelSelector: opts.LabelSelector}

	// Pods
	podList, err := opts.Clientset.CoreV1().Pods(ns).List(ctx, listOpts)
	if err != nil {
		return nil, fmt.Errorf("listing pods: %w", err)
	}
	for _, p := range podList.Items {
		topo.Pods = append(topo.Pods, podFromK8s(p))
	}

	// Services
	svcList, err := opts.Clientset.CoreV1().Services(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing services: %w", err)
	}

	// Endpoints for service→pod mapping
	epList, err := opts.Clientset.DiscoveryV1().EndpointSlices(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing endpoint slices: %w", err)
	}
	epMap := buildEndpointMap(epList.Items)

	for _, s := range svcList.Items {
		si := serviceFromK8s(s)
		key := s.Namespace + "/" + s.Name
		si.BackingPods = resolvePodNames(epMap[key], topo.Pods)
		topo.Services = append(topo.Services, si)
	}

	// NetworkPolicies
	if opts.IncludePolicies {
		npList, err := opts.Clientset.NetworkingV1().NetworkPolicies(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("listing networkpolicies: %w", err)
		}
		for _, np := range npList.Items {
			topo.Policies = append(topo.Policies, policyFromK8s(np))
		}
	}

	// Ingresses
	if opts.IncludeIngress {
		ingList, err := opts.Clientset.NetworkingV1().Ingresses(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("listing ingresses: %w", err)
		}
		for _, ing := range ingList.Items {
			topo.Ingresses = append(topo.Ingresses, ingressFromK8s(ing))
		}
	}

	return topo, nil
}

// ─── Conversion helpers ───────────────────────────────────────────────────────

func podFromK8s(p corev1.Pod) PodInfo {
	pi := PodInfo{
		Name:      p.Name,
		Namespace: p.Namespace,
		IP:        p.Status.PodIP,
		NodeName:  p.Spec.NodeName,
		Labels:    p.Labels,
		Phase:     string(p.Status.Phase),
		CreatedAt: p.CreationTimestamp.Time,
	}
	for _, c := range p.Status.Conditions {
		if c.Type == corev1.PodReady {
			pi.Ready = c.Status == corev1.ConditionTrue
		}
	}
	for _, container := range p.Spec.Containers {
		for _, port := range container.Ports {
			pi.Ports = append(pi.Ports, PortInfo{
				Name:     port.Name,
				Port:     port.ContainerPort,
				Protocol: string(port.Protocol),
			})
		}
	}
	return pi
}

func serviceFromK8s(s corev1.Service) ServiceInfo {
	si := ServiceInfo{
		Name:      s.Name,
		Namespace: s.Namespace,
		ClusterIP: s.Spec.ClusterIP,
		Type:      string(s.Spec.Type),
		Selector:  s.Spec.Selector,
		CreatedAt: s.CreationTimestamp.Time,
	}
	for _, lb := range s.Status.LoadBalancer.Ingress {
		if lb.IP != "" {
			si.ExternalIP = lb.IP
		} else if lb.Hostname != "" {
			si.ExternalIP = lb.Hostname
		}
	}
	for _, p := range s.Spec.Ports {
		sp := ServicePort{
			Name:     p.Name,
			Port:     p.Port,
			Protocol: string(p.Protocol),
			NodePort: p.NodePort,
		}
		if p.TargetPort.Type == 0 {
			sp.TargetPort = fmt.Sprintf("%d", p.TargetPort.IntVal)
		} else {
			sp.TargetPort = p.TargetPort.StrVal
		}
		si.Ports = append(si.Ports, sp)
	}
	return si
}

func policyFromK8s(np networkingv1.NetworkPolicy) NetworkPolicyInfo {
	npi := NetworkPolicyInfo{
		Name:        np.Name,
		Namespace:   np.Namespace,
		PodSelector: np.Spec.PodSelector.MatchLabels,
		SelectsAll:  len(np.Spec.PodSelector.MatchLabels) == 0 && len(np.Spec.PodSelector.MatchExpressions) == 0,
	}
	for _, pt := range np.Spec.PolicyTypes {
		npi.PolicyTypes = append(npi.PolicyTypes, string(pt))
	}
	for _, ir := range np.Spec.Ingress {
		rule := NetworkPolicyRule{}
		for _, p := range ir.Ports {
			if p.Port != nil {
				rule.Ports = append(rule.Ports, p.Port.IntVal)
			}
		}
		if len(ir.From) == 0 {
			rule.AllowAllPods = true
			rule.AllowAllNamespaces = true
		}
		for _, from := range ir.From {
			if from.NamespaceSelector != nil {
				if len(from.NamespaceSelector.MatchLabels) == 0 {
					rule.AllowAllNamespaces = true
				} else {
					rule.FromNamespaces = append(rule.FromNamespaces, labelsToString(from.NamespaceSelector.MatchLabels))
				}
			}
			if from.PodSelector != nil {
				rule.FromPodSelectors = append(rule.FromPodSelectors, from.PodSelector.MatchLabels)
			}
			if from.IPBlock != nil {
				rule.IPBlocks = append(rule.IPBlocks, from.IPBlock.CIDR)
			}
		}
		npi.IngressRules = append(npi.IngressRules, rule)
	}
	for _, er := range np.Spec.Egress {
		rule := NetworkPolicyRule{}
		for _, p := range er.Ports {
			if p.Port != nil {
				rule.Ports = append(rule.Ports, p.Port.IntVal)
			}
		}
		if len(er.To) == 0 {
			rule.AllowAllPods = true
			rule.AllowAllNamespaces = true
		}
		for _, to := range er.To {
			if to.NamespaceSelector != nil {
				if len(to.NamespaceSelector.MatchLabels) == 0 {
					rule.AllowAllNamespaces = true
				} else {
					rule.ToNamespaces = append(rule.ToNamespaces, labelsToString(to.NamespaceSelector.MatchLabels))
				}
			}
			if to.PodSelector != nil {
				rule.ToPodSelectors = append(rule.ToPodSelectors, to.PodSelector.MatchLabels)
			}
			if to.IPBlock != nil {
				rule.IPBlocks = append(rule.IPBlocks, to.IPBlock.CIDR)
			}
		}
		npi.EgressRules = append(npi.EgressRules, rule)
	}
	return npi
}

func ingressFromK8s(ing networkingv1.Ingress) IngressInfo {
	ii := IngressInfo{
		Name:      ing.Name,
		Namespace: ing.Namespace,
		TLS:       len(ing.Spec.TLS) > 0,
		CreatedAt: ing.CreationTimestamp.Time,
	}
	for _, t := range ing.Spec.TLS {
		ii.TLSHOSTS = append(ii.TLSHOSTS, t.Hosts...)
	}
	for _, r := range ing.Spec.Rules {
		if r.HTTP == nil {
			continue
		}
		for _, path := range r.HTTP.Paths {
			ir := IngressRule{
				Host:     r.Host,
				Path:     path.Path,
				PathType: string(*path.PathType),
			}
			if path.Backend.Service != nil {
				ir.ServiceName = path.Backend.Service.Name
				ir.ServicePort = path.Backend.Service.Port.Number
			}
			ii.Rules = append(ii.Rules, ir)
		}
	}
	return ii
}

// ─── Endpoint helpers ─────────────────────────────────────────────────────────

type endpointEntry struct {
	IPs []string
}

func buildEndpointMap(slices []discoveryv1.EndpointSlice) map[string]endpointEntry {
	m := make(map[string]endpointEntry)
	for _, slice := range slices {
		svcName := slice.Labels["kubernetes.io/service-name"]
		if svcName == "" {
			continue
		}
		key := slice.Namespace + "/" + svcName
		entry := m[key]
		for _, ep := range slice.Endpoints {
			entry.IPs = append(entry.IPs, ep.Addresses...)
		}
		m[key] = entry
	}
	return m
}

func resolvePodNames(ep endpointEntry, pods []PodInfo) []string {
	ipToPod := make(map[string]string)
	for _, p := range pods {
		if p.IP != "" {
			ipToPod[p.IP] = p.Namespace + "/" + p.Name
		}
	}
	var result []string
	for _, ip := range ep.IPs {
		if name, ok := ipToPod[ip]; ok {
			result = append(result, name)
		}
	}
	return result
}

// ─── Connectivity Analysis ───────────────────────────────────────────────────

// ConnectivityResult holds the result of a policy-based connectivity check.
type ConnectivityResult struct {
	Allowed        bool
	Reason         string
	MatchedIngress []string
	MatchedEgress  []string
	TraceSteps     []TraceStep
}

// TraceStep is one step in the connectivity trace explanation.
type TraceStep struct {
	Step    int
	Title   string
	Allowed *bool // nil = neutral/info
	Detail  string
}

// CheckConnectivity performs a static NetworkPolicy analysis to determine whether
// srcPod (namespace/name) can reach dstPod (namespace/name) on the given port.
// port == 0 means any port.
func CheckConnectivity(topo *Topology, srcRef, dstRef string, port int32) ConnectivityResult {
	res := ConnectivityResult{}
	boolPtr := func(b bool) *bool { return &b }

	step := func(title, detail string, allowed *bool) {
		n := len(res.TraceSteps) + 1
		res.TraceSteps = append(res.TraceSteps, TraceStep{Step: n, Title: title, Detail: detail, Allowed: allowed})
	}

	srcPod := findPod(topo, srcRef)
	dstPod := findPod(topo, dstRef)

	if srcPod == nil {
		step("Resolve source pod", fmt.Sprintf("Pod %q not found in scan", srcRef), boolPtr(false))
		res.Reason = fmt.Sprintf("source pod %q not found", srcRef)
		return res
	}
	if dstPod == nil {
		step("Resolve destination pod", fmt.Sprintf("Pod %q not found in scan", dstRef), boolPtr(false))
		res.Reason = fmt.Sprintf("destination pod %q not found", dstRef)
		return res
	}

	step("Resolve source pod", fmt.Sprintf("%s/%s  IP=%s  namespace=%s", srcPod.Namespace, srcPod.Name, srcPod.IP, srcPod.Namespace), nil)
	step("Resolve destination pod", fmt.Sprintf("%s/%s  IP=%s  namespace=%s", dstPod.Namespace, dstPod.Name, dstPod.IP, dstPod.Namespace), nil)
	if port > 0 {
		step("Target port", fmt.Sprintf("port %d", port), nil)
	}

	// ── Egress check on source ────────────────────────────────────────────────
	srcPolicies := policiesSelectingPod(topo, srcPod, "Egress")
	if len(srcPolicies) == 0 {
		step("Egress check", "No NetworkPolicy selects the source pod for egress → egress unrestricted", boolPtr(true))
	} else {
		step("Egress check", fmt.Sprintf("%d NetworkPolicy/ies restrict egress from source pod", len(srcPolicies)), nil)
		egressAllowed := false
		for _, p := range srcPolicies {
			for _, r := range p.EgressRules {
				if ruleMatchesDst(r, dstPod, port) {
					egressAllowed = true
					res.MatchedEgress = append(res.MatchedEgress, p.Namespace+"/"+p.Name)
					step("Egress rule match", fmt.Sprintf("Policy %s/%s allows egress to %s/%s", p.Namespace, p.Name, dstPod.Namespace, dstPod.Name), boolPtr(true))
				}
			}
		}
		if !egressAllowed {
			step("Egress blocked", "No egress rule in any matching policy allows traffic to destination", boolPtr(false))
			res.Reason = "egress blocked by NetworkPolicy"
			return res
		}
	}

	// ── Ingress check on destination ─────────────────────────────────────────
	dstPolicies := policiesSelectingPod(topo, dstPod, "Ingress")
	if len(dstPolicies) == 0 {
		step("Ingress check", "No NetworkPolicy selects the destination pod for ingress → ingress unrestricted", boolPtr(true))
		res.Allowed = true
		res.Reason = "no restrictive NetworkPolicy — traffic is allowed"
		return res
	}

	step("Ingress check", fmt.Sprintf("%d NetworkPolicy/ies restrict ingress to destination pod", len(dstPolicies)), nil)
	for _, p := range dstPolicies {
		for _, r := range p.IngressRules {
			if ruleMatchesSrc(r, srcPod, port) {
				res.MatchedIngress = append(res.MatchedIngress, p.Namespace+"/"+p.Name)
				step("Ingress rule match", fmt.Sprintf("Policy %s/%s allows ingress from %s/%s", p.Namespace, p.Name, srcPod.Namespace, srcPod.Name), boolPtr(true))
				res.Allowed = true
				res.Reason = fmt.Sprintf("allowed by ingress rule in NetworkPolicy %s/%s", p.Namespace, p.Name)
				return res
			}
		}
	}

	step("Ingress blocked", "No ingress rule allows traffic from the source pod", boolPtr(false))
	res.Reason = "ingress blocked by NetworkPolicy"
	return res
}

// ─── Audit helpers ────────────────────────────────────────────────────────────

// NamespaceAudit summarises the isolation level of a namespace.
type NamespaceAudit struct {
	Namespace     string
	PolicyCount   int
	HasIngress    bool
	HasEgress     bool
	ExposedPods   []string // pods not selected by any policy
	CoverageLevel string   // "none", "partial", "full"
}

// AuditNamespaces returns isolation audits for each namespace in the topology.
func AuditNamespaces(topo *Topology) []NamespaceAudit {
	nsSet := make(map[string]struct{})
	for _, p := range topo.Pods {
		nsSet[p.Namespace] = struct{}{}
	}
	for _, s := range topo.Services {
		nsSet[s.Namespace] = struct{}{}
	}

	var audits []NamespaceAudit
	for ns := range nsSet {
		audit := NamespaceAudit{Namespace: ns}
		for _, p := range topo.Policies {
			if p.Namespace != ns {
				continue
			}
			audit.PolicyCount++
			for _, pt := range p.PolicyTypes {
				if pt == "Ingress" {
					audit.HasIngress = true
				}
				if pt == "Egress" {
					audit.HasEgress = true
				}
			}
		}
		// Find pods not selected by any policy
		for _, pod := range topo.Pods {
			if pod.Namespace != ns {
				continue
			}
			selected := false
			for _, p := range topo.Policies {
				if p.Namespace != ns {
					continue
				}
				if podMatchesSelector(pod.Labels, p.PodSelector) {
					selected = true
					break
				}
			}
			if !selected {
				audit.ExposedPods = append(audit.ExposedPods, pod.Name)
			}
		}
		switch {
		case audit.PolicyCount == 0:
			audit.CoverageLevel = "none"
		case len(audit.ExposedPods) > 0:
			audit.CoverageLevel = "partial"
		default:
			audit.CoverageLevel = "full"
		}
		audits = append(audits, audit)
	}
	return audits
}

// ─── Internal helpers ─────────────────────────────────────────────────────────

func findPod(topo *Topology, ref string) *PodInfo {
	parts := strings.SplitN(ref, "/", 2)
	for i := range topo.Pods {
		p := &topo.Pods[i]
		if len(parts) == 2 {
			if p.Namespace == parts[0] && p.Name == parts[1] {
				return p
			}
		} else {
			if p.Name == parts[0] {
				return p
			}
		}
	}
	return nil
}

func policiesSelectingPod(topo *Topology, pod *PodInfo, policyType string) []NetworkPolicyInfo {
	var result []NetworkPolicyInfo
	for _, np := range topo.Policies {
		if np.Namespace != pod.Namespace {
			continue
		}
		applies := false
		for _, pt := range np.PolicyTypes {
			if pt == policyType {
				applies = true
				break
			}
		}
		if !applies {
			continue
		}
		if np.SelectsAll || podMatchesSelector(pod.Labels, np.PodSelector) {
			result = append(result, np)
		}
	}
	return result
}

func podMatchesSelector(podLabels, selector map[string]string) bool {
	if len(selector) == 0 {
		return true
	}
	sel := labels.Set(selector)
	return sel.AsSelector().Matches(labels.Set(podLabels))
}

func ruleMatchesSrc(rule NetworkPolicyRule, src *PodInfo, port int32) bool {
	if !portMatches(rule.Ports, port) {
		return false
	}
	if rule.AllowAllPods && rule.AllowAllNamespaces {
		return true
	}
	for _, sel := range rule.FromPodSelectors {
		if podMatchesSelector(src.Labels, sel) {
			return true
		}
	}
	return rule.AllowAllNamespaces
}

func ruleMatchesDst(rule NetworkPolicyRule, dst *PodInfo, port int32) bool {
	if !portMatches(rule.Ports, port) {
		return false
	}
	if rule.AllowAllPods && rule.AllowAllNamespaces {
		return true
	}
	for _, sel := range rule.ToPodSelectors {
		if podMatchesSelector(dst.Labels, sel) {
			return true
		}
	}
	return rule.AllowAllNamespaces
}

func portMatches(rulePorts []int32, port int32) bool {
	if len(rulePorts) == 0 || port == 0 {
		return true
	}
	for _, rp := range rulePorts {
		if rp == port {
			return true
		}
	}
	return false
}

func labelsToString(m map[string]string) string {
	var parts []string
	for k, v := range m {
		parts = append(parts, k+"="+v)
	}
	return strings.Join(parts, ",")
}
