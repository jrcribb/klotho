package path_selection

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/dominikbraun/graph"
	construct "github.com/klothoplatform/klotho/pkg/construct"
	engine_errs "github.com/klothoplatform/klotho/pkg/engine/errors"
	"github.com/klothoplatform/klotho/pkg/engine/solution"
	knowledgebase "github.com/klothoplatform/klotho/pkg/knowledgebase"
	"github.com/klothoplatform/klotho/pkg/logging"
	"github.com/klothoplatform/klotho/pkg/set"
)

//go:generate mockgen -source=./path_expansion.go --destination=../operational_eval/path_expansion_mock_test.go --package=operational_eval

type (
	ExpansionInput struct {
		ExpandEdge       construct.SimpleEdge
		SatisfactionEdge construct.ResourceEdge
		Classification   string
		TempGraph        construct.Graph
	}
	ExpansionResult struct {
		Edges []graph.Edge[construct.ResourceId]
		Graph construct.Graph
	}

	EdgeExpander interface {
		ExpandEdge(input ExpansionInput) (ExpansionResult, error)
	}

	EdgeExpand struct {
		Ctx solution.Solution
	}
)

func (e *EdgeExpand) ExpandEdge(
	input ExpansionInput,
) (ExpansionResult, error) {
	ctx := e.Ctx
	tempGraph := input.TempGraph
	dep := input.SatisfactionEdge

	result := ExpansionResult{
		Graph: construct.NewGraph(),
	}

	defer writeGraph(ctx.Context(), input, tempGraph, result.Graph)
	var errs error
	// TODO: Revisit if we want to run on namespaces (this causes issue depending on what the namespace is)
	// A file system can be a namespace and that doesnt really fit the reason we are running this at the moment
	// errs = errors.Join(errs, runOnNamespaces(dep.Source, dep.Target, ctx, result))
	connected, err := connectThroughNamespace(dep.Source, dep.Target, ctx, result)
	if err != nil {
		errs = errors.Join(errs, err)
	}
	if !connected {
		edges, err := expandEdge(ctx, input, result.Graph)
		errs = errors.Join(errs, err)
		result.Edges = append(result.Edges, edges...)
	}
	return result, errs
}

func expandEdge(
	ctx solution.Solution,
	input ExpansionInput,
	g construct.Graph,
) ([]graph.Edge[construct.ResourceId], error) {
	paths, err := graph.AllPathsBetween(input.TempGraph, input.SatisfactionEdge.Source.ID, input.SatisfactionEdge.Target.ID)
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, engine_errs.UnsupportedExpansionErr{
			ExpandEdge: input.ExpandEdge,
			SatisfactionEdge: construct.SimpleEdge{
				Source: input.SatisfactionEdge.Source.ID,
				Target: input.SatisfactionEdge.Target.ID,
			},
			Classification: input.Classification,
		}
	}
	sort.Slice(paths, func(i, j int) bool {
		il, jl := len(paths[i]), len(paths[j])
		if il != jl {
			return il < jl
		}
		pi, pj := paths[i], paths[j]
		for k := 0; k < il; k++ {
			if pi[k] != pj[k] {
				return construct.ResourceIdLess(pi[k], pj[k])
			}
		}
		return false
	})

	undirected, err := BuildUndirectedGraph(ctx.RawView(), ctx.KnowledgeBase())
	if err != nil {
		return nil, err
	}

	var errs error
	// represents id to qualified type because we dont need to do that processing more than once
	for _, path := range paths {
		err := expandPath(ctx, undirected, input, path, g)
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("error expanding path %s: %w", construct.Path(path), err))
		}
	}
	if errs != nil {
		return nil, errs
	}

	path, err := graph.ShortestPathStable(
		input.TempGraph,
		input.SatisfactionEdge.Source.ID,
		input.SatisfactionEdge.Target.ID,
		construct.ResourceIdLess,
	)
	if err != nil {
		// NOTE(gg) this can't happen with the current expandPath implementation
		// but may in the future.
		return nil, engine_errs.InvalidPathErr{
			ExpandEdge: input.ExpandEdge,
			SatisfactionEdge: construct.SimpleEdge{
				Source: input.SatisfactionEdge.Source.ID,
				Target: input.SatisfactionEdge.Target.ID,
			},
			Classification: input.Classification,
		}
	}

	resultResources, err := renameAndReplaceInTempGraph(ctx, input, g, path)
	errs = errors.Join(errs, err)
	edges, err := findSubExpansionsToRun(resultResources, ctx)
	return edges, errors.Join(errs, err)
}

func renameAndReplaceInTempGraph(
	ctx solution.Solution,
	input ExpansionInput,
	g construct.Graph,
	path construct.Path,
) ([]*construct.Resource, error) {
	var errs error
	name := fmt.Sprintf("%s-%s", input.SatisfactionEdge.Source.ID.Name, input.SatisfactionEdge.Target.ID.Name)
	// rename phantom nodes
	result := make([]*construct.Resource, len(path))
	for i, id := range path {
		r, props, err := input.TempGraph.VertexWithProperties(id)
		if err != nil {
			errs = errors.Join(errs, err)
			continue
		}
		if strings.HasPrefix(id.Name, PHANTOM_PREFIX) {
			id.Name = name
			// because certain resources may be namespaced, we will check against all resource type names
			currNames, err := getCurrNames(ctx, &id)
			if err != nil {
				errs = errors.Join(errs, err)
				continue
			}
			for suffix := 0; suffix < 1000; suffix++ {
				if !currNames.Contains(id.Name) {
					break
				}
				id.Name = fmt.Sprintf("%s-%d", name, suffix)
			}
			if props.Attributes != nil {
				props.Attributes["new_id"] = id.String()
			}
			phantomRes := r
			r, err = knowledgebase.CreateResource(ctx.KnowledgeBase(), id)
			if err != nil {
				errs = errors.Join(errs, err)
				continue
			}
			r.Properties = phantomRes.Properties
			phantomRes.Properties = make(construct.Properties)
		}
		err = g.AddVertex(r)
		switch {
		case errors.Is(err, graph.ErrVertexAlreadyExists):
			r, err = g.Vertex(id)
			if err != nil {
				errs = errors.Join(errs, err)
				continue
			}

		case err != nil:
			errs = errors.Join(errs, err)
			continue
		}
		result[i] = r
		if i > 0 {
			err = g.AddEdge(result[i-1].ID, r.ID)
			if err != nil && !errors.Is(err, graph.ErrEdgeAlreadyExists) {
				errs = errors.Join(errs, fmt.Errorf("error adding edge for path[%d]: %w", i, err))
			}
		}
	}
	if errs != nil {
		return nil, errs
	}

	// We need to replace the phantom nodes in the temp graph in case we reuse the temp graph for sub expansions
	for i, res := range result {
		err := construct.ReplaceResource(input.TempGraph, path[i], res)
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("error replacing path[%d] %s: %w", i, path[i], err))
		}
	}
	return result, errs
}

func getCurrNames(sol solution.Solution, resourceToSet *construct.ResourceId) (set.Set[string], error) {
	currNames := make(set.Set[string])
	ids, err := construct.TopologicalSort(sol.DataflowGraph())
	if err != nil {
		return currNames, err
	}
	// we cannot consider things only in the namespace because when creating a resource for an operational action
	// it likely has not been namespaced yet and we dont know where it will be namespaced to
	matcher := construct.ResourceId{Provider: resourceToSet.Provider, Type: resourceToSet.Type}
	for _, id := range ids {
		if matcher.Matches(id) {
			currNames.Add(id.Name)
		}
	}
	return currNames, nil
}

func findSubExpansionsToRun(
	result []*construct.Resource,
	ctx solution.Solution,
) (edges []graph.Edge[construct.ResourceId], errs error) {
	resourceTemplates := make(map[construct.ResourceId]*knowledgebase.ResourceTemplate)
	added := make(map[construct.ResourceId]map[construct.ResourceId]bool)
	getResourceTemplate := func(id construct.ResourceId) *knowledgebase.ResourceTemplate {
		rt, found := resourceTemplates[id]
		if !found {
			var err error
			rt, err = ctx.KnowledgeBase().GetResourceTemplate(id)
			if err != nil || rt == nil {
				errs = errors.Join(errs, fmt.Errorf("could not find resource template for %s: %w", id, err))
				return nil
			}
		}
		return rt
	}

	for i, res := range result {
		if i == 0 || i == len(result)-1 {
			continue
		}
		rt := getResourceTemplate(res.ID)
		if rt == nil {
			continue
		}
		if len(rt.PathSatisfaction.AsSource) != 0 {
			for j := i + 2; j < len(result); j++ {
				target := result[j]
				rt := getResourceTemplate(target.ID)
				if rt == nil {
					continue
				}
				if len(rt.PathSatisfaction.AsTarget) != 0 || j == len(result)-1 {
					if _, ok := added[res.ID]; !ok {
						added[res.ID] = make(map[construct.ResourceId]bool)
					}
					if added, ok := added[res.ID][target.ID]; !ok || !added {
						edges = append(edges, graph.Edge[construct.ResourceId]{Source: res.ID, Target: target.ID})
					}
					added[res.ID][target.ID] = true
				}
			}
		}
		// do the same logic for asTarget
		if len(rt.PathSatisfaction.AsTarget) != 0 {
			for j := i - 2; j >= 0; j-- {
				source := result[j]
				rt := getResourceTemplate(source.ID)
				if rt == nil {
					continue
				}
				if len(rt.PathSatisfaction.AsSource) != 0 || j == 0 {
					if _, ok := added[source.ID]; !ok {
						added[source.ID] = make(map[construct.ResourceId]bool)
					}
					if added, ok := added[source.ID][res.ID]; !ok || !added {
						edges = append(edges, graph.Edge[construct.ResourceId]{Source: source.ID, Target: res.ID})
					}
					added[source.ID][res.ID] = true
				}
			}
		}
	}
	return
}

// ExpandEdge takes a given `selectedPath` and resolves it to a path of resourceIds that can be used
// for creating resources, or existing resources.
// 'undirected' is the undirected graph of the dataflow graph from 'ctx' but are a separate input to reuse
// the calculated graph for performance.
func expandPath(
	ctx solution.Solution,
	undirected construct.Graph,
	input ExpansionInput,
	path construct.Path,
	resultGraph construct.Graph,
) error {
	log := logging.GetLogger(ctx.Context()).Sugar()

	if len(path) == 2 {
		modifiesImport, err := checkModifiesImportedResource(input.SatisfactionEdge.Source.ID,
			input.SatisfactionEdge.Target.ID, ctx, nil)
		if err != nil {
			return err
		}
		if modifiesImport {
			// Because the direct edge will cause modifications to an imported resource, we need to remove the direct edge
			return input.TempGraph.RemoveEdge(input.SatisfactionEdge.Source.ID,
				input.SatisfactionEdge.Target.ID)
		}
	}
	log.Debugf("Resolving path %s", path)

	type candidate struct {
		id             construct.ResourceId
		divideWeightBy int
	}

	var errs error

	nonBoundaryResources := path[1 : len(path)-1]

	// candidates maps the nonboundary index to the set of resources that could satisfy it
	// this is a helper to make adding all the edges to the graph easier.
	candidates := make([]map[construct.ResourceId]int, len(nonBoundaryResources))

	newResources := make(set.Set[construct.ResourceId])
	// Create new nodes for the path
	for i, node := range nonBoundaryResources {
		candidates[i] = make(map[construct.ResourceId]int)
		candidates[i][node] = 0
		resource, err := input.TempGraph.Vertex(node)
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("error getting vertex for path[%d]: %w", i, err))
			continue
		}
		// we know phantoms are always able to be valid, so we want to ensure we make them valid based on src and target validity checks
		// right now we dont want validity checks to be blocking, just preference so we use them to modify the weight
		valid, err := checkCandidatesValidity(ctx, resource, path, input.Classification)
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("error checking validity of path[%d]: %w", i, err))
			continue
		}
		if !valid {
			candidates[i][node] = -1000
		}
		newResources.Add(node)
	}
	if errs != nil {
		return errs
	}

	addCandidates := func(id construct.ResourceId, resource *construct.Resource, nerr error) error {
		matchIdx := matchesNonBoundary(id, nonBoundaryResources)
		if matchIdx < 0 {
			return nil
		}
		valid, err := checkNamespaceValidity(ctx, resource, input.SatisfactionEdge.Target.ID)
		if err != nil {
			return errors.Join(nerr, fmt.Errorf("error checking namespace validity of %s: %w", resource.ID, err))
		}
		if !valid {
			return nerr
		}

		// Calculate edge weight for candidate
		err = input.TempGraph.AddVertex(resource)
		if err != nil && !errors.Is(err, graph.ErrVertexAlreadyExists) {
			return errors.Join(nerr, err)
		}
		if _, ok := candidates[matchIdx][id]; !ok {
			candidates[matchIdx][id] = 0
		}
		weight, err := determineCandidateWeight(ctx, input.SatisfactionEdge.Source.ID, input.SatisfactionEdge.Target.ID, id, resultGraph, undirected)
		if err != nil {
			return errors.Join(nerr, err)
		}

		// right now we dont want validity checks to be blocking, just preference so we use them to modify the weight
		valid, err = checkCandidatesValidity(ctx, resource, path, input.Classification)
		if err != nil {
			return errors.Join(nerr, err)
		}
		if !valid {
			weight = -1000
		}
		candidates[matchIdx][id] += weight
		return nerr
	}
	// We need to add candidates which exist in our current result graph so we can reuse them. We do this in case
	// we have already performed expansions to ensure the namespaces are connected, etc
	err := construct.WalkGraph(resultGraph, func(id construct.ResourceId, resource *construct.Resource, nerr error) error {
		return addCandidates(id, resource, nerr)
	})
	if err != nil {
		errs = errors.Join(errs, fmt.Errorf("error during result graph walk graph: %w", err))
	}

	// Add all other candidates which exist within the graph
	err = construct.WalkGraph(ctx.RawView(), func(id construct.ResourceId, resource *construct.Resource, nerr error) error {
		return addCandidates(id, resource, nerr)
	})
	if err != nil {
		errs = errors.Join(errs, fmt.Errorf("error during raw view walk graph: %w", err))
	}

	edges, err := ctx.DataflowGraph().Edges()
	if err != nil {
		errs = errors.Join(errs, err)
	}
	if errs != nil {
		return errs
	}

	// addEdge checks whether the edge should be added according to the following rules:
	// 1. If it connects two new resources, always add it
	// 2. If the edge exists, and its template specifies it is unique, only add it if it's an existing edge
	// 3. Otherwise, add it
	addEdge := func(source, target candidate) {
		weight := CalculateEdgeWeight(
			construct.SimpleEdge{Source: input.SatisfactionEdge.Source.ID, Target: input.SatisfactionEdge.Target.ID},
			source.id, target.id,
			source.divideWeightBy, target.divideWeightBy,
			input.Classification,
			ctx.KnowledgeBase())

		tmpl := ctx.KnowledgeBase().GetEdgeTemplate(source.id, target.id)
		if tmpl == nil {
			errs = errors.Join(errs, fmt.Errorf("could not find edge template for %s -> %s", source.id, target.id))
			return
		}
		if !tmpl.Unique.CanAdd(edges, source.id, target.id) {
			return
		}
		modifiesImport, err := checkModifiesImportedResource(source.id, target.id, ctx, tmpl)
		if err != nil {
			errs = errors.Join(errs, err)
			return
		}
		if modifiesImport {
			return
		}
		// if the edge doesnt exist in the actual graph and there is any uniqueness constraint,
		// then we need to check uniqueness validity
		_, err = ctx.RawView().Edge(source.id, target.id)
		if errors.Is(err, graph.ErrEdgeNotFound) {
			if tmpl.Unique.Source || tmpl.Unique.Target {
				valid, err := checkUniquenessValidity(ctx, source.id, target.id)
				if err != nil {
					errs = errors.Join(errs, err)
					return
				}
				if !valid {
					return
				}
			}
		} else if err != nil {
			errs = errors.Join(errs, fmt.Errorf("unexpected error from raw edge: %v", err))
			return
		}

		err = input.TempGraph.AddEdge(source.id, target.id, graph.EdgeWeight(weight))
		switch {
		case errors.Is(err, graph.ErrEdgeAlreadyExists):
			errs = errors.Join(errs, input.TempGraph.UpdateEdge(source.id, target.id, graph.EdgeWeight(weight)))

		case errors.Is(err, graph.ErrEdgeCreatesCycle):
			// ignore cycles

		case err != nil:
			errs = errors.Join(errs, fmt.Errorf("unexpected error adding edge to temp graph: %v", err))
		}
	}

	for i, resCandidates := range candidates {
		for id, weight := range resCandidates {
			if i == 0 {
				addEdge(candidate{id: input.SatisfactionEdge.Source.ID}, candidate{id: id, divideWeightBy: weight})
				continue
			}

			sources := candidates[i-1]

			for source, w := range sources {
				addEdge(candidate{id: source, divideWeightBy: w}, candidate{id: id, divideWeightBy: weight})
			}

		}
	}
	if len(candidates) > 0 {
		for c, weight := range candidates[len(candidates)-1] {
			addEdge(candidate{id: c, divideWeightBy: weight}, candidate{id: input.SatisfactionEdge.Target.ID})
		}
	}
	if errs != nil {
		return errs
	}
	return nil
}

func connectThroughNamespace(src, target *construct.Resource, sol solution.Solution, result ExpansionResult) (
	connected bool,
	errs error,
) {
	kb := sol.KnowledgeBase()
	targetNamespaceResource, _ := kb.GetResourcesNamespaceResource(target)
	if targetNamespaceResource.IsZero() {
		return
	}

	downstreams, err := solution.Downstream(sol, src.ID, knowledgebase.ResourceLocalLayer)
	if err != nil {
		return connected, err
	}
	for _, downId := range downstreams {
		// Right now we only check for side effects of the same type
		// We may want to check for any side effects that could be namespaced into the target namespace since that would influence
		// the source resources connection to that target namespace resource
		if downId.QualifiedTypeName() != targetNamespaceResource.QualifiedTypeName() {
			continue
		}
		down, err := sol.RawView().Vertex(downId)
		if err != nil {
			errs = errors.Join(errs, err)
			continue
		}
		res, _ := kb.GetResourcesNamespaceResource(down)
		if res.IsZero() {
			continue
		}
		if res == targetNamespaceResource {
			continue
		}
		// if we have a namespace resource that is not the same as the target namespace resource
		tg, err := BuildPathSelectionGraph(
			sol.Context(),
			construct.SimpleEdge{Source: res, Target: target.ID},
			kb,
			"",
			true,
		)
		if err != nil {
			continue
		}
		input := ExpansionInput{
			SatisfactionEdge: construct.ResourceEdge{Source: down, Target: target},
			Classification:   "",
			TempGraph:        tg,
		}
		edges, err := expandEdge(sol, input, result.Graph)
		if err != nil {
			errs = errors.Join(errs, err)
			continue
		}
		result.Edges = append(result.Edges, edges...)
		connected = true
	}

	return
}

// NOTE(gg): if for some reason the path could contain a duplicated selector
// this would just add the resource to the first match. I don't
// think this should happen for a call into [ExpandEdge], but noting it just in case.
func matchesNonBoundary(id construct.ResourceId, nonBoundaryResources []construct.ResourceId) int {
	for i, node := range nonBoundaryResources {
		typedNodeId := construct.ResourceId{Provider: node.Provider, Type: node.Type, Namespace: node.Namespace}
		if typedNodeId.Matches(id) {
			return i
		}
	}
	return -1
}
