package deps

type NodeType string

const (
	NodeSource   NodeType = "source"
	NodeDocument NodeType = "document"
	NodeTemplate NodeType = "template"
	NodeData     NodeType = "data"
	NodeTaxonomy NodeType = "taxonomy"
	NodeOutput   NodeType = "output"
)

type Node struct {
	ID   string         `json:"id"`
	Type NodeType       `json:"type"`
	Meta map[string]any `json:"meta"`
}

type Edge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type Graph struct {
	nodes   map[string]*Node
	forward map[string]map[string]struct{}
	reverse map[string]map[string]struct{}
}

func NewGraph() *Graph {
	return &Graph{
		nodes:   make(map[string]*Node),
		forward: make(map[string]map[string]struct{}),
		reverse: make(map[string]map[string]struct{}),
	}
}

func (g *Graph) AddNode(n *Node) {
	if n == nil || n.ID == "" {
		return
	}
	if _, ok := g.nodes[n.ID]; !ok {
		g.nodes[n.ID] = n
	}
}

func (g *Graph) AddEdge(from, to string) {
	if from == "" || to == "" {
		return
	}
	if _, ok := g.forward[from]; !ok {
		g.forward[from] = make(map[string]struct{})
	}
	if _, ok := g.reverse[to]; !ok {
		g.reverse[to] = make(map[string]struct{})
	}
	g.forward[from][to] = struct{}{}
	g.reverse[to][from] = struct{}{}
}

func (g *Graph) Node(id string) (*Node, bool) {
	n, ok := g.nodes[id]
	return n, ok
}

func (g *Graph) DependentsOf(id string) []string {
	seen := make(map[string]struct{})
	queue := []string{id}
	out := make([]string, 0)

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		for dep := range g.forward[cur] {
			if _, ok := seen[dep]; ok {
				continue
			}
			seen[dep] = struct{}{}
			out = append(out, dep)
			queue = append(queue, dep)
		}
	}

	return out
}

func (g *Graph) Export() map[string]any {
	nodes := make([]*Node, 0, len(g.nodes))
	edges := make([]Edge, 0)

	for _, n := range g.nodes {
		nodes = append(nodes, n)
	}
	for from, tos := range g.forward {
		for to := range tos {
			edges = append(edges, Edge{From: from, To: to})
		}
	}

	return map[string]any{
		"nodes": nodes,
		"edges": edges,
	}
}
