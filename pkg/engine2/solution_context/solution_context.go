package solution_context

import (
	"fmt"
	"reflect"

	"github.com/dominikbraun/graph"
	"github.com/klothoplatform/klotho/pkg/collectionutil"
	construct "github.com/klothoplatform/klotho/pkg/construct2"
	"github.com/klothoplatform/klotho/pkg/engine2/constraints"
	constructexpansion "github.com/klothoplatform/klotho/pkg/engine2/construct_expansion"
	"github.com/klothoplatform/klotho/pkg/engine2/operational_rule"
	"github.com/klothoplatform/klotho/pkg/engine2/path_selection"
	knowledgebase "github.com/klothoplatform/klotho/pkg/knowledge_base2"
)

type (
	SolutionContext struct {
		dataflowGraph        construct.Graph
		deploymentGraph      construct.Graph
		decisions            DecisionRecords
		stack                []KV
		kb                   knowledgebase.TemplateKB
		mappedResources      map[construct.ResourceId]construct.ResourceId
		CreateResourcefromId func(id construct.ResourceId) *construct.Resource
		EdgeConstraints      []constraints.EdgeConstraint
		ResourceConstraints  []constraints.ResourceConstraint
		ConstructConstraints []constraints.ConstructConstraint
	}
	AddResourceDecision struct {
		Resource construct.ResourceId
	}

	RemoveResourceDecision struct {
		Resource construct.ResourceId
	}

	AddDependencyDecision struct {
		From construct.ResourceId
		To   construct.ResourceId
	}

	RemoveDependencyDecision struct {
		From construct.ResourceId
		To   construct.ResourceId
	}

	SetPropertyDecision struct {
		Resource construct.ResourceId
		Property string
		Value    any
	}
)

func NewSolutionContext() SolutionContext {
	return SolutionContext{
		dataflowGraph:   construct.NewGraph(),
		deploymentGraph: construct.NewAcyclicGraph(),
		decisions:       &MemoryRecord{},
	}
}

func (ctx SolutionContext) LoadGraph(graph construct.Graph) error {
	err := construct.WalkGraph(graph, func(id construct.ResourceId, resource *construct.Resource, nerr error) error {
		if nerr != nil {
			return nerr
		}
		ctx.addResource(resource, false)
		return nil
	})
	if err != nil {
		return err
	}
	edges, err := graph.Edges()
	if err != nil {
		return err
	}
	for _, edge := range edges {
		src, err := graph.Vertex(edge.Source)
		if err != nil {
			return err
		}
		target, err := graph.Vertex(edge.Target)
		if err != nil {
			return err
		}
		ctx.addDependency(src, target, false)
	}
	return nil
}

func (c SolutionContext) Clone() SolutionContext {
	dfClone, err := c.dataflowGraph.Clone()
	if err != nil {
		panic(err)
	}
	deployClone, err := c.deploymentGraph.Clone()
	if err != nil {
		panic(err)
	}
	return SolutionContext{
		dataflowGraph:   dfClone,
		deploymentGraph: deployClone,
		decisions:       c.decisions,
	}
}

func (s SolutionContext) With(key string, value any) SolutionContext {
	return SolutionContext{
		dataflowGraph:   s.dataflowGraph,
		deploymentGraph: s.deploymentGraph,
		decisions:       s.decisions,

		stack: append(s.stack, KV{key: key, value: value}),
	}
}

func (c SolutionContext) GetDecisions() DecisionRecords {
	return c.decisions
}

// RecordDecision snapshots the current stack and records the decision
func (c SolutionContext) RecordDecision(d SolveDecision) {
	c.decisions.AddRecord(c.stack, d)
}

func (ctx SolutionContext) Solve() error {
	err := construct.WalkGraph(ctx.dataflowGraph, func(id construct.ResourceId, resource *construct.Resource, nerr error) error {
		if nerr != nil {
			return nerr
		}
		err := ctx.nodeMakeOperational(resource)
		if err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return err
	}

	edges, err := ctx.dataflowGraph.Edges()
	if err != nil {
		return err
	}
	for _, dep := range edges {
		err := ctx.edgeMakeOperational(dep)
		if err != nil {
			return err
		}
	}
	return nil
}

func (ctx SolutionContext) GetConstructsResource(constructId construct.ResourceId) *construct.Resource {
	res, _ := ctx.GetResource(ctx.mappedResources[constructId])
	return res
}

func (ctx SolutionContext) nodeMakeOperational(r *construct.Resource) error {

	ctx = ctx.With("resource", r) // add the resource to the context stack

	// handle resource constraints before to prevent unnecessary actions

	template, err := ctx.kb.GetResourceTemplate(r.ID)
	if err != nil {
		panic(err)
	}
	for _, property := range template.Properties {
		if property.OperationalStep == nil {
			continue
		}
		ruleCtx := operational_rule.OperationalRuleContext{
			Property:             &property,
			ConfigCtx:            knowledgebase.ConfigTemplateContext{DAG: ctx.dataflowGraph},
			Data:                 knowledgebase.ConfigTemplateData{Resource: r.ID},
			Graph:                ctx,
			KB:                   ctx.kb,
			CreateResourcefromId: ctx.CreateResourcefromId,
		}
		err := ruleCtx.HandleOperationalStep(*property.OperationalStep)
		if err != nil {
			return err
		}
	}
	return nil
}

func (ctx SolutionContext) edgeMakeOperational(e graph.Edge[construct.ResourceId]) error {
	ctx = ctx.With("edge", e) // add the edge info to the decision context stack

	template := ctx.kb.GetEdgeTemplate(e.Source, e.Target)
	for _, rule := range template.OperationalRules {
		ruleCtx := operational_rule.OperationalRuleContext{
			ConfigCtx:            knowledgebase.ConfigTemplateContext{DAG: ctx.dataflowGraph},
			Data:                 knowledgebase.ConfigTemplateData{Edge: e},
			Graph:                ctx,
			KB:                   ctx.kb,
			CreateResourcefromId: ctx.CreateResourcefromId,
		}
		err := ruleCtx.HandleOperationalRule(rule)
		if err != nil {
			return err
		}
	}
	return nil
}

func (ctx SolutionContext) addPath(from, to *construct.Resource) error {
	dep, err := ctx.dataflowGraph.Edge(from.ID, to.ID)
	if err != nil {
		return err
	}
	ctx.With("edge", dep)
	pathCtx := path_selection.PathSelectionContext{
		Graph:                ctx,
		KB:                   ctx.kb,
		CreateResourcefromId: ctx.CreateResourcefromId,
	}

	// Find any edge constraints around path selection
	edgeData := path_selection.EdgeData{}
	for _, constraint := range ctx.EdgeConstraints {
		if constraint.Target.Source == from.ID && constraint.Target.Target == to.ID {
			if constraint.Operator == constraints.MustContainConstraintOperator {
				edgeData.Constraint.NodeMustExist = append(edgeData.Constraint.NodeMustExist, *ctx.CreateResourcefromId(constraint.Node))
			} else if constraint.Operator == constraints.MustNotContainConstraintOperator {
				edgeData.Constraint.NodeMustNotExist = append(edgeData.Constraint.NodeMustNotExist, *ctx.CreateResourcefromId(constraint.Node))
			} else if constraint.Operator == constraints.EqualsConstraintOperator {
				for key, val := range constraint.Attributes {
					edgeData.Attributes[key] = val
				}
			}
		}
	}

	edges, err := pathCtx.SelectPath(dep, edgeData)
	if err != nil {
		return err
	}
	if len(edges) == 1 {
		err := ctx.edgeMakeOperational(graph.Edge[construct.ResourceId]{Source: from.ID, Target: to.ID})
		if err != nil {
			return err
		}
		return nil
	} else {
		err := ctx.RemoveDependency(from.ID, to.ID)
		if err != nil {
			return err
		}
	}
	for _, edge := range edges {
		err := ctx.edgeMakeOperational(graph.Edge[construct.ResourceId]{Source: edge.Source.ID, Target: edge.Target.ID})
		if err != nil {
			return err
		}
	}
	return nil
}

func (ctx SolutionContext) ConfigureResource(resource *construct.Resource, configuration knowledgebase.Configuration, data knowledgebase.ConfigTemplateData) error {
	if resource == nil {
		return fmt.Errorf("resource does not exist")
	}
	configCtx := knowledgebase.ConfigTemplateContext{DAG: ctx.dataflowGraph}
	newConfig, err := configCtx.ResolveConfig(configuration, data)
	if err != nil {
		return err
	}
	err = ConfigureField(resource, newConfig.Field, newConfig.Value, false, ctx.dataflowGraph)
	if err != nil {
		return err
	}
	ctx.RecordDecision(SetPropertyDecision{
		Resource: resource.Id(),
		Property: configuration.Field,
		Value:    configuration.Value,
	})
	return nil
}

func (ctx SolutionContext) ExpandConstruct(resource *construct.Resource, constraints []constraints.ConstructConstraint) ([]SolutionContext, error) {
	expCtx := constructexpansion.ConstructExpansionContext{
		Construct:            resource,
		Kb:                   ctx.kb,
		CreateResourceFromId: ctx.CreateResourcefromId,
	}
	solutions, err := expCtx.ExpandConstruct(resource, constraints)
	if err != nil {
		return nil, err
	}
	result := []SolutionContext{}
	for _, solution := range solutions {
		newCtx := ctx.Clone()
		newCtx.With("construct", resource)
		for _, edge := range solution.Edges {
			newCtx.AddDependency(&edge.Source, &edge.Target)
		}
		res, err := newCtx.GetResource(solution.DirectlyMappedResource)
		if err != nil {
			return nil, err
		}
		newCtx.ReplaceResourceId(resource.ID, res)
		result = append(result, newCtx)
	}
	return result, nil
}

func (ctx SolutionContext) GenerateCombinations() ([]SolutionContext, error) {
	solutions := []SolutionContext{ctx}
	resources, err := ctx.ListResources()
	if err != nil {
		return nil, err
	}
	for _, res := range resources {
		if res.ID.IsAbstractResource() {
			newSolutions := []SolutionContext{}
			for _, sol := range solutions {
				ctxs, err := sol.ExpandConstruct(res, []constraints.ConstructConstraint{})
				if err != nil {
					return nil, err
				}
				newSolutions = append(newSolutions, ctxs...)
			}
			solutions = newSolutions
		}
	}
	return solutions, nil
}

func (ctx SolutionContext) GetClassification(resource construct.ResourceId) knowledgebase.Classification {
	return ctx.kb.GetClassification(resource)
}

func (ctx SolutionContext) GetFunctionality(resource construct.ResourceId) knowledgebase.Functionality {
	return ctx.kb.GetFunctionality(resource)
}

func (d AddResourceDecision) internal()      {}
func (d AddDependencyDecision) internal()    {}
func (d RemoveResourceDecision) internal()   {}
func (d RemoveDependencyDecision) internal() {}
func (d SetPropertyDecision) internal()      {}

func (ctx SolutionContext) isOperationalResourceSideEffect(resource, sideEffect *construct.Resource) bool {
	template, err := ctx.kb.GetResourceTemplate(resource.ID)
	if template == nil || err != nil {
		return false
	}
	for _, property := range template.Properties {
		ruleSatisfied := false
		if property.OperationalStep == nil {
			continue
		}
		rule := property.OperationalStep
		if rule.Resources != nil {
			resources, types, err := property.OperationalStep.ExtractResourcesAndTypes(knowledgebase.ConfigTemplateContext{DAG: ctx.dataflowGraph}, knowledgebase.ConfigTemplateData{Resource: resource.ID})
			if err != nil {
				continue
			}
			if collectionutil.Contains(types, construct.ResourceId{Provider: sideEffect.ID.Provider, Type: sideEffect.ID.Type}) {
				ruleSatisfied = true
			}
			if collectionutil.Contains(resources, sideEffect.ID) {
				ruleSatisfied = true
			}
		}
		if rule.Classifications != nil {
			if template.ResourceContainsClassifications(rule.Classifications) {
				ruleSatisfied = true
			}
		}

		// If the side effect resource fits the rule we then perform 2 more checks
		// 1. is there a path in the direction of the rule
		// 2. Is the property set with the resource that we are checking for
		if ruleSatisfied {
			if rule.Direction == knowledgebase.Upstream {
				resources, err := graph.ShortestPath(ctx.dataflowGraph, sideEffect.ID, resource.ID)
				if len(resources) == 0 || err != nil {
					continue
				}
			} else {
				resources, err := graph.ShortestPath(ctx.dataflowGraph, resource.ID, sideEffect.ID)
				if len(resources) == 0 || err != nil {
					continue
				}
			}

			val, _, err := parseFieldName(resource, property.Name, ctx.dataflowGraph, false)
			if err != nil {
				return false
			}
			if val.Kind() == reflect.Array || val.Kind() == reflect.Slice {
				for i := 0; i < val.Len(); i++ {
					if val.Index(i).Interface().(construct.Resource).ID == sideEffect.ID {
						return true
					}
				}
			} else {
				if val.Interface().(*construct.Resource).ID == sideEffect.ID {
					return true
				}
			}

		}
	}
	return false
}
