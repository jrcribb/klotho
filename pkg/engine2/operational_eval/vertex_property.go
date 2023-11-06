package operational_eval

import (
	"errors"
	"fmt"

	construct "github.com/klothoplatform/klotho/pkg/construct2"
	"github.com/klothoplatform/klotho/pkg/engine2/constraints"
	"github.com/klothoplatform/klotho/pkg/engine2/operational_rule"
	"github.com/klothoplatform/klotho/pkg/engine2/solution_context"
	knowledgebase "github.com/klothoplatform/klotho/pkg/knowledge_base2"
	"go.uber.org/zap"
)

type (
	propertyVertex struct {
		Ref construct.PropertyRef

		Template  *knowledgebase.Property
		EdgeRules map[construct.SimpleEdge][]knowledgebase.OperationalRule
	}
)

func (prop propertyVertex) Key() Key {
	return Key{Ref: prop.Ref}
}

func (prop *propertyVertex) Dependencies(ctx solution_context.SolutionContext) (vertexDependencies, error) {
	cfgCtx := solution_context.DynamicCtx(ctx)
	propCtx := &fauxConfigContext{
		propRef: prop.Ref,
		inner:   cfgCtx,
		deps:    newDeps(),
	}

	resData := knowledgebase.DynamicValueData{Resource: prop.Ref.Resource}

	// Template can be nil when checking for dependencies from a propertyVertex when adding an edge template
	if prop.Template != nil {
		if err := propCtx.ExecuteValue(prop.Template.DefaultValue, resData); err != nil {
			return propCtx.deps, fmt.Errorf("could not execute default value template for %s: %w", prop.Ref, err)
		}

		if opRule := prop.Template.OperationalRule; opRule != nil {
			if err := propCtx.ExecuteOpRule(resData, *opRule); err != nil {
				return propCtx.deps, fmt.Errorf("could not execute resource operational rule for %s: %w", prop.Ref, err)
			}
		}

		if !prop.Template.Namespace {
			tmpl, err := cfgCtx.KB().GetResourceTemplate(prop.Ref.Resource)
			if err != nil {
				return propCtx.deps, fmt.Errorf("could not get resource template for %s: %w", prop.Ref.Resource, err)
			}
			for propKey, propTmpl := range tmpl.Properties {
				if propTmpl.Namespace {
					nsRef := construct.PropertyRef{Resource: prop.Ref.Resource, Property: propKey}
					propCtx.deps.Outgoing.Add(Key{Ref: nsRef})
				}
			}
		}
	}

	for edge, rules := range prop.EdgeRules {
		var errs error
		edgeData := knowledgebase.DynamicValueData{
			Resource: prop.Ref.Resource,
			Edge:     &construct.Edge{Source: edge.Source, Target: edge.Target},
		}
		for _, rule := range rules {
			errs = errors.Join(errs, propCtx.ExecuteOpRule(edgeData, rule))
		}
		if errs != nil {
			return propCtx.deps, fmt.Errorf(
				"could not execute %s for edge %s: %w",
				prop.Ref, edge.Source, errs,
			)
		}
	}

	return propCtx.deps, nil
}

func (prop *propertyVertex) UpdateFrom(otherV Vertex) {
	if prop == otherV {
		return
	}
	other, ok := otherV.(*propertyVertex)
	if !ok {
		panic(fmt.Sprintf("cannot merge property with non-property vertex: %T", otherV))
	}
	if prop.Ref != other.Ref {
		panic(fmt.Sprintf("cannot merge properties with different refs: %s != %s", prop.Ref, other.Ref))
	}

	if prop.Template == nil {
		prop.Template = other.Template
	}
	if prop.EdgeRules == nil {
		prop.EdgeRules = make(map[construct.SimpleEdge][]knowledgebase.OperationalRule)
	}

	for edge, rules := range other.EdgeRules {
		if _, ok := prop.EdgeRules[edge]; ok {
			// already have rules for this edge, don't duplicate them
			continue
		}
		prop.EdgeRules[edge] = rules
	}
}

func (v *propertyVertex) Evaluate(eval *Evaluator) error {
	zap.S().With("op", "eval").Debugf("Evaluating %s", v.Ref)

	sol := eval.Solution.With("resource", v.Ref.Resource).With("property", v.Ref.Property)

	res, err := sol.RawView().Vertex(v.Ref.Resource)
	if err != nil {
		return fmt.Errorf("could not get resource to evaluate %s: %w", v.Ref, err)
	}

	if err := v.evaluateConstraints(sol, res); err != nil {
		return err
	}

	if err := v.evaluateResourceOperational(sol, res); err != nil {
		return err
	}

	if err := v.evaluateEdgeOperational(sol, res); err != nil {
		return err
	}

	if err := eval.UpdateId(v.Ref.Resource, res.ID); err != nil {
		return err
	}

	return nil
}

func (v *propertyVertex) evaluateConstraints(sol solution_context.SolutionContext, res *construct.Resource) error {
	dynData := knowledgebase.DynamicValueData{Resource: res.ID}

	var setConstraint constraints.ResourceConstraint
	var addConstraints []constraints.ResourceConstraint
	for _, c := range sol.Constraints().Resources {
		if c.Target != res.ID || c.Property != v.Ref.Property {
			continue
		}
		if c.Operator == constraints.EqualsConstraintOperator {
			setConstraint = c
			continue
		}
		addConstraints = append(addConstraints, c)
	}
	currentValue, err := res.GetProperty(v.Ref.Property)
	if err != nil {
		return fmt.Errorf("could not get current value for %s: %w", v.Ref, err)
	}

	if currentValue == nil && setConstraint.Operator == "" && v.Template != nil && v.Template.DefaultValue != nil {
		err = solution_context.ConfigureResource(
			sol,
			res,
			knowledgebase.Configuration{Field: v.Ref.Property, Value: v.Template.DefaultValue},
			dynData,
			"set",
		)
		if err != nil {
			return fmt.Errorf("could not set default value for %s: %w", v.Ref, err)
		}
	} else if setConstraint.Operator != "" {
		kb := sol.KnowledgeBase()
		resTemplate, err := kb.GetResourceTemplate(res.ID)
		if err != nil {
			return fmt.Errorf("could not get resource template for %s: %w", res.ID, err)
		}
		property := resTemplate.GetProperty(v.Ref.Property)
		if property == nil {
			return fmt.Errorf("could not get property %s from resource %s", v.Ref.Property, res.ID)
		}
		switch setConstraint.Operator {
		case constraints.AddConstraintOperator, constraints.RemoveConstraintOperator:
			if property.IsPropertyTypeScalar() {
				return fmt.Errorf("cannot add/remove to scalar property %s", v.Ref)
			}
		}
		err = solution_context.ConfigureResource(
			sol,
			res,
			knowledgebase.Configuration{Field: v.Ref.Property, Value: setConstraint.Value},
			dynData,
			"set",
		)
		if err != nil {
			return fmt.Errorf("could not apply initial constraint for %s: %w", v.Ref, err)
		}
	}
	dynData.Resource = res.ID // Update in case the property changes the ID

	var errs error
	for _, c := range addConstraints {
		if c.Operator == constraints.EqualsConstraintOperator {
			continue
		}
		action, err := solution_context.ConstraintOperatorToAction(c.Operator)
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("could not apply constraint for %s: %w", v.Ref, err))
			continue
		}
		errs = errors.Join(errs, solution_context.ConfigureResource(
			sol,
			res,
			knowledgebase.Configuration{Field: v.Ref.Property, Value: c.Value},
			dynData,
			action,
		))
		dynData.Resource = res.ID
	}
	if errs != nil {
		return fmt.Errorf("could not apply constraints for %s: %w", v.Ref, errs)
	}

	return nil
}

func (v *propertyVertex) evaluateResourceOperational(sol solution_context.SolutionContext, res *construct.Resource) error {
	if v.Template == nil || v.Template.OperationalRule == nil {
		return nil
	}

	opCtx := operational_rule.OperationalRuleContext{
		Solution: sol,
		Property: v.Template,
		Data:     knowledgebase.DynamicValueData{Resource: res.ID},
	}

	err := opCtx.HandleOperationalRule(*v.Template.OperationalRule)
	if err != nil {
		return fmt.Errorf("could not apply operational rule for %s: %w", v.Ref, err)
	}

	return nil
}

func (v *propertyVertex) evaluateEdgeOperational(sol solution_context.SolutionContext, res *construct.Resource) error {
	oldId := v.Ref.Resource

	opCtx := operational_rule.OperationalRuleContext{
		Solution: sol,
		Property: v.Template,
		Data:     knowledgebase.DynamicValueData{Resource: res.ID},
	}

	var errs error
	for edge, rules := range v.EdgeRules {
		for _, rule := range rules {
			// In case one of the previous rules changed the ID, update it
			edge = UpdateEdgeId(edge, oldId, res.ID)

			opCtx.Data.Edge = &construct.Edge{
				Source: edge.Source,
				Target: edge.Target,
			}
			err := opCtx.HandleOperationalRule(rule)
			if err != nil {
				errs = errors.Join(errs, fmt.Errorf(
					"could not apply edge %s -> %s operational rule for %s: %w",
					edge.Source, edge.Target, v.Ref, err,
				))
			}
		}
	}
	return errs
}
