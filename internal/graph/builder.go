// Package graph builds and renders Kubernetes network topology graphs.
package graph

import (
	"fmt"
	"strings"

	"github.com/themark007/knew/internal/k8s"
)

// NodeType classifies graph nodes.
type NodeType string

const (
	NodePod      NodeType = "pod"
	NodeService  NodeType = "service"
	NodeIngress  NodeType = "ingress"
	NodeExternal NodeType = "external"
)

// EdgeType classifies graph edges.
type EdgeType string

const (
	EdgeEndpoint     EdgeType = "endpoint"     // service → pod via endpoints
	EdgeSelector     EdgeType = "selector"     // service → pod via label selector
	EdgeIngressRoute EdgeType = "ingress-route" // ingress → service
	EdgePolicyAllow  EdgeType = "policy-allow" // allowed by NetworkPolicy
)

// Node is a vertex in the network graph.
type Node struct {
	ID        string
	Name      string
	Namespace string
	Type      NodeType
	Labels    map[string]string
	Extra     string // additional text shown in detail view
}

// Edge is a directed edge between two nodes.
type Edge struct {
	From  string
	To    string
	Type  EdgeType
	Label string
}

// Graph is an adjacency-list network graph.
type Graph struct {
	Nodes []Node
	Edges []Edge
	// adjacency lookup
	nodeIndex map[string]int
	outEdges  map[string][]int
	inEdges   map[string][]int
}

// Build constructs a Graph from a Topology.
func Build(topo *k8s.Topology) *Graph {
	g := &Graph{
		nodeIndex: make(map[string]int),
		outEdges:  make(map[string][]int),
		inEdges:   make(map[string][]int),
	}

	// Add pod nodes
	for _, p := range topo.Pods {
		id := nodeID(NodePod, p.Namespace, p.Name)
		ports := make([]string, len(p.Ports))
		for i, pt := range p.Ports {
			ports[i] = fmt.Sprintf("%d/%s", pt.Port, pt.Protocol)
		}
		extra := fmt.Sprintf("IP: %s | Node: %s | Phase: %s | Ports: %s",
			p.IP, p.NodeName, p.Phase, strings.Join(ports, ", "))
		g.addNode(Node{ID: id, Name: p.Name, Namespace: p.Namespace, Type: NodePod, Labels: p.Labels, Extra: extra})
	}

	// Add service nodes + edges
	for _, s := range topo.Services {
		id := nodeID(NodeService, s.Namespace, s.Name)
		ports := make([]string, len(s.Ports))
		for i, sp := range s.Ports {
			ports[i] = fmt.Sprintf("%d→%s", sp.Port, sp.TargetPort)
		}
		extra := fmt.Sprintf("ClusterIP: %s | Type: %s | Ports: %s",
			s.ClusterIP, s.Type, strings.Join(ports, ", "))
		g.addNode(Node{ID: id, Name: s.Name, Namespace: s.Namespace, Type: NodeService, Labels: s.Selector, Extra: extra})

		// Service → backing pods
		for _, podRef := range s.BackingPods {
			parts := strings.SplitN(podRef, "/", 2)
			var podNS, podName string
			if len(parts) == 2 {
				podNS, podName = parts[0], parts[1]
			} else {
				podNS, podName = s.Namespace, parts[0]
			}
			podID := nodeID(NodePod, podNS, podName)
			g.addEdge(Edge{From: id, To: podID, Type: EdgeEndpoint, Label: "endpoint"})
		}
	}

	// Add ingress nodes + edges
	for _, ing := range topo.Ingresses {
		id := nodeID(NodeIngress, ing.Namespace, ing.Name)
		g.addNode(Node{ID: id, Name: ing.Name, Namespace: ing.Namespace, Type: NodeIngress, Extra: fmt.Sprintf("TLS: %v", ing.TLS)})
		for _, r := range ing.Rules {
			if r.ServiceName == "" {
				continue
			}
			svcID := nodeID(NodeService, ing.Namespace, r.ServiceName)
			g.addEdge(Edge{From: id, To: svcID, Type: EdgeIngressRoute, Label: r.Host + r.Path})
		}
	}

	// Build adjacency index
	for i, e := range g.Edges {
		g.outEdges[e.From] = append(g.outEdges[e.From], i)
		g.inEdges[e.To] = append(g.inEdges[e.To], i)
	}

	return g
}

// NodeByID returns a node pointer by ID, or nil.
func (g *Graph) NodeByID(id string) *Node {
	if i, ok := g.nodeIndex[id]; ok {
		return &g.Nodes[i]
	}
	return nil
}

// OutNeighbours returns nodes reachable from nodeID.
func (g *Graph) OutNeighbours(id string) []Node {
	var result []Node
	for _, ei := range g.outEdges[id] {
		e := g.Edges[ei]
		if n := g.NodeByID(e.To); n != nil {
			result = append(result, *n)
		}
	}
	return result
}

// InNeighbours returns nodes that point to nodeID.
func (g *Graph) InNeighbours(id string) []Node {
	var result []Node
	for _, ei := range g.inEdges[id] {
		e := g.Edges[ei]
		if n := g.NodeByID(e.From); n != nil {
			result = append(result, *n)
		}
	}
	return result
}

// OutEdgesFrom returns outgoing edges from a node.
func (g *Graph) OutEdgesFrom(id string) []Edge {
	var result []Edge
	for _, ei := range g.outEdges[id] {
		result = append(result, g.Edges[ei])
	}
	return result
}

// Stats returns a summary string.
func (g *Graph) Stats() string {
	pods, svcs, ings := 0, 0, 0
	for _, n := range g.Nodes {
		switch n.Type {
		case NodePod:
			pods++
		case NodeService:
			svcs++
		case NodeIngress:
			ings++
		}
	}
	return fmt.Sprintf("nodes=%d (pods=%d services=%d ingresses=%d) edges=%d",
		len(g.Nodes), pods, svcs, ings, len(g.Edges))
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func nodeID(t NodeType, ns, name string) string {
	return string(t) + ":" + ns + "/" + name
}

func (g *Graph) addNode(n Node) {
	if _, exists := g.nodeIndex[n.ID]; !exists {
		g.nodeIndex[n.ID] = len(g.Nodes)
		g.Nodes = append(g.Nodes, n)
	}
}

func (g *Graph) addEdge(e Edge) {
	// Ensure target node exists (as placeholder)
	if _, ok := g.nodeIndex[e.To]; !ok {
		// Create a minimal placeholder that will be enriched later
		parts := strings.SplitN(e.To, ":", 2)
		if len(parts) == 2 {
			nsName := strings.SplitN(parts[1], "/", 2)
			var ns, name string
			if len(nsName) == 2 {
				ns, name = nsName[0], nsName[1]
			} else {
				name = nsName[0]
			}
			g.addNode(Node{ID: e.To, Name: name, Namespace: ns, Type: NodeType(parts[0])})
		}
	}
	g.Edges = append(g.Edges, e)
}
