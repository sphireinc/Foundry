package deps

type ChangeSet struct {
	Sources   []string
	Templates []string
	DataKeys  []string
	Assets    []string
	Full      bool
}

type RebuildPlan struct {
	OutputURLs  []string
	FullRebuild bool
}

func ResolveRebuildPlan(g *Graph, changes ChangeSet) RebuildPlan {
	if changes.Full {
		return RebuildPlan{FullRebuild: true}
	}

	affected := make(map[string]struct{})

	addDependents := func(nodeID string) {
		for _, dep := range g.DependentsOf(nodeID) {
			affected[dep] = struct{}{}
		}
	}

	for _, s := range changes.Sources {
		addDependents(sourceNodeID(s))
	}
	for _, t := range changes.Templates {
		addDependents(templateNodeID(t))
	}
	for _, d := range changes.DataKeys {
		addDependents(dataNodeID(d))
	}

	plan := RebuildPlan{
		OutputURLs: make([]string, 0),
	}

	for id := range affected {
		n, ok := g.Node(id)
		if !ok {
			continue
		}
		if n.Type == NodeOutput {
			if v, ok := n.Meta["url"].(string); ok {
				plan.OutputURLs = append(plan.OutputURLs, v)
			}
		}
	}

	return plan
}
