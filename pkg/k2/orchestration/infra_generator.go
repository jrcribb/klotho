package orchestration

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/klothoplatform/klotho/pkg/construct"
	"github.com/klothoplatform/klotho/pkg/engine"
	"github.com/klothoplatform/klotho/pkg/engine/constraints"
	engine_errs "github.com/klothoplatform/klotho/pkg/engine/errors"
	solution_context "github.com/klothoplatform/klotho/pkg/engine/solution"
	"github.com/klothoplatform/klotho/pkg/infra/iac"
	kio "github.com/klothoplatform/klotho/pkg/io"
	"github.com/klothoplatform/klotho/pkg/knowledgebase"
	"github.com/klothoplatform/klotho/pkg/knowledgebase/reader"
	"github.com/klothoplatform/klotho/pkg/provider/aws"
	"github.com/klothoplatform/klotho/pkg/templates"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

type (
	InfraGenerator struct {
		Engine *engine.Engine
	}

	InfraRequest struct {
		Provider    string
		InputGraph  construct.Graph
		Constraints constraints.Constraints
		OutputDir   string
		GlobalTag   string
	}
)

var cachedEngine *engine.Engine

func (g *InfraGenerator) addEngine() error {
	if cachedEngine != nil {
		g.Engine = cachedEngine
		return nil
	}

	kb, err := reader.NewKBFromFs(templates.ResourceTemplates, templates.EdgeTemplates, templates.Models)
	if err != nil {
		return err
	}
	cachedEngine = engine.NewEngine(kb)
	g.Engine = cachedEngine
	return nil
}

func (g *InfraGenerator) Run(c constraints.Constraints, outDir string) error {
	// TODO the engine currently assumes only 1 run globally, so the debug graphs and other files
	// will get overwritten with each run. We should fix this.
	engineContext, errs := g.resolveResources(InfraRequest{
		Provider:    "aws",
		Constraints: c,
		OutputDir:   outDir,
		GlobalTag:   "k2",
	})
	if errs != nil {
		return fmt.Errorf("failed to resolve resources: %v", errs)
	}

	err := g.generateIac(iacRequest{
		PulumiAppName: "k2",
		Context:       engineContext,
		OutputDir:     outDir,
	})
	if err != nil {
		return fmt.Errorf("failed to generate iac: %w", err)

	}
	return nil
}

func (g *InfraGenerator) resolveResources(request InfraRequest) (*engine.SolveRequest, []engine_errs.EngineError) {
	var engErrs []engine_errs.EngineError
	internalError := func(err error) {
		engErrs = append(engErrs, engine_errs.InternalError{Err: err})
	}
	log := zap.S().Named("engine")

	defer func() { // defer functions execute in FILO order, so this executes after the 'recover'.
		err := writeEngineErrsJson(engErrs, os.Stdout)
		if err != nil {
			log.Errorf("failed to output errors to stdout: %v", err)
		}
	}()
	defer func() {
		r := recover()
		if r == nil {
			return
		}
		log.Errorf("panic: %v", r)
		switch r := r.(type) {
		case engine_errs.EngineError:
			engErrs = append(engErrs, r)
		case error:
			engErrs = append(engErrs, engine_errs.InternalError{Err: r})
		default:
			engErrs = append(engErrs, engine_errs.InternalError{Err: fmt.Errorf("panic: %v", r)})
		}
	}()

	err := g.addEngine()
	if err != nil {
		internalError(err)
		return nil, engErrs
	}

	context := &engine.SolveRequest{
		GlobalTag: "k2", // TODO: consider making this configurable
	}

	if request.InputGraph != nil {
		clonedGraph, err := request.InputGraph.Clone()
		if err != nil {
			internalError(fmt.Errorf("failed to clone graph: %w", err))
			return nil, engErrs
		}
		context.InitialState = clonedGraph
	} else {
		context.InitialState = construct.NewGraph()
	}
	log.Info("Loading constraints")

	context.Constraints = request.Constraints
	// len(engErrs) == 0 at this point so overwriting it is safe
	// All other assignments prior are via 'internalError' and return
	exitCode, engErrs := g.runEngine(context)
	if exitCode == 1 {
		return nil, engErrs
	}

	var files []kio.File

	configErrors := new(bytes.Buffer)
	err = writeEngineErrsJson(engErrs, configErrors)
	if err != nil {
		internalError(fmt.Errorf("failed to write config errors: %w", err))
		return nil, engErrs
	}
	files = append(files, &kio.RawFile{
		FPath:   "config_errors.json",
		Content: configErrors.Bytes(),
	})

	log.Info("Engine finished running... Generating views")
	vizFiles, err := g.Engine.VisualizeViews(context.Solutions[0])
	if err != nil {
		internalError(fmt.Errorf("failed to generate views %w", err))
		return nil, engErrs
	}
	files = append(files, vizFiles...)
	log.Info("Generating resources.yaml")
	b, err := yaml.Marshal(construct.YamlGraph{Graph: context.Solutions[0].DataflowGraph(), Outputs: context.Solutions[0].Outputs()})
	if err != nil {
		internalError(fmt.Errorf("failed to marshal graph: %w", err))
		return nil, engErrs
	}
	files = append(files,
		&kio.RawFile{
			FPath:   "resources.yaml",
			Content: b,
		},
	)

	if request.Provider == "aws" {
		policyBytes, err := aws.DeploymentPermissionsPolicy(context.Solutions[0])
		if err != nil {
			internalError(fmt.Errorf("failed to generate deployment permissions policy: %w", err))
			return nil, engErrs
		}
		files = append(files,
			&kio.RawFile{
				FPath:   "deployment_permissions_policy.json",
				Content: policyBytes,
			},
		)
	}

	err = kio.OutputTo(files, request.OutputDir)
	if err != nil {
		internalError(fmt.Errorf("failed to write output files: %w", err))
		return nil, engErrs
	}
	return context, engErrs
}

func writeEngineErrsJson(errs []engine_errs.EngineError, out io.Writer) error {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	// NOTE: since this isn't used in a web context (it's a CLI), we can disable escaping.
	enc.SetEscapeHTML(false)

	outErrs := make([]map[string]any, len(errs))
	for i, e := range errs {
		outErrs[i] = e.ToJSONMap()
		outErrs[i]["error_code"] = e.ErrorCode()
		wrapped := errors.Unwrap(e)
		if wrapped != nil {
			outErrs[i]["error"] = engine_errs.ErrorsToTree(wrapped)
		}
	}
	return enc.Encode(outErrs)
}

func (g *InfraGenerator) runEngine(context *engine.SolveRequest) (int, []engine_errs.EngineError) {
	returnCode := 0
	var engErrs []engine_errs.EngineError

	log := zap.S().Named("engine")

	log.Info("Running engine")
	err := g.Engine.Run(context)
	if err != nil {
		// When the engine returns an error, that indicates that it halted evaluation, thus is a fatal error.
		// This is returned as exit code 1, and add the details to be printed to stdout.
		returnCode = 1
		engErrs = append(engErrs, extractEngineErrors(err)...)
		log.Errorf("Engine returned error: %v", err)
	}

	if len(context.Solutions) > 0 {
		writeDebugGraphs(context.Solutions[0])

		// If there are any decisions that are engine errors, add them to the list of error details
		// to be printed to stdout. These are returned as exit code 2 unless it is already code 1.
		for _, d := range context.Solutions[0].GetDecisions().GetRecords() {
			d, ok := d.(solution_context.AsEngineError)
			if !ok {
				continue
			}
			ee := d.TryEngineError()
			if ee == nil {
				continue
			}
			engErrs = append(engErrs, ee)
			if returnCode != 1 {
				returnCode = 2
			}
		}
	}

	return returnCode, engErrs
}

func extractEngineErrors(err error) []engine_errs.EngineError {
	if err == nil {
		return nil
	}
	var errs []engine_errs.EngineError
	queue := []error{err}
	for len(queue) > 0 {
		err := queue[0]
		queue = queue[1:]
		switch err := err.(type) {
		case engine_errs.EngineError:
			errs = append(errs, err)
		case interface{ Unwrap() []error }:
			queue = append(queue, err.Unwrap()...)
		case interface{ Unwrap() error }:
			queue = append(queue, err.Unwrap())
		}
	}
	if len(errs) == 0 {
		errs = append(errs, engine_errs.InternalError{Err: err})
	}
	return errs
}

func writeDebugGraphs(sol solution.Solution) {
	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		defer wg.Done()
		err := engine.GraphToSVG(sol.KnowledgeBase(), sol.DataflowGraph(), "dataflow")
		if err != nil {
			zap.S().Named("engine").Errorf("failed to write dataflow graph: %w", err)
		}
	}()
	go func() {
		defer wg.Done()
		err := engine.GraphToSVG(sol.KnowledgeBase(), sol.DeploymentGraph(), "iac")
		if err != nil {
			zap.S().Named("engine").Errorf("failed to write iac graph: %w", err)
		}
	}()
	wg.Wait()
}

type iacRequest struct {
	PulumiAppName string
	Context       *engine.SolveRequest
	OutputDir     string
}

var cachedKb *knowledgebase.KnowledgeBase

func (g *InfraGenerator) generateIac(request iacRequest) error {
	var files []kio.File

	solCtx := request.Context.Solutions[0]
	var kb *knowledgebase.KnowledgeBase
	var err error
	if cachedKb == nil {
		kb, err = reader.NewKBFromFs(templates.ResourceTemplates, templates.EdgeTemplates, templates.Models)
		if err != nil {
			return err
		}
		cachedKb = kb
	}
	kb = cachedKb

	pulumiPlugin := iac.Plugin{
		Config: &iac.PulumiConfig{AppName: request.PulumiAppName},
		KB:     kb,
	}
	iacFiles, err := pulumiPlugin.Translate(solCtx)
	if err != nil {
		return err
	}
	files = append(files, iacFiles...)

	err = kio.OutputTo(files, request.OutputDir)
	if err != nil {
		return err
	}

	return nil
}
