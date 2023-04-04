package compiler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/klothoplatform/klotho/pkg/annotation"
	"github.com/klothoplatform/klotho/pkg/core"
	"github.com/pelletier/go-toml/v2"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"
)

type (
	Plugin interface {
		Name() string
	}

	AnalysisAndTransformationPlugin interface {
		Plugin
		// Transform is expected to mutate the result and any dependencies
		Transform(*core.InputFiles, *core.ConstructGraph) error
	}

	ProviderPlugin interface {
		Plugin
		Translate(result *core.ConstructGraph, dag *core.ResourceGraph) ([]core.CloudResourceLink, error)
	}

	IaCPlugin interface {
		Plugin
		Translate(cloudGraph *core.ResourceGraph) ([]core.File, error)
	}

	Compiler struct {
		AnalysisAndTransformationPlugins []AnalysisAndTransformationPlugin
		ProviderPlugins                  []ProviderPlugin
		IaCPlugins                       []IaCPlugin
		Document                         CompilationDocument
	}

	// ResourcesOrErr provided as commonly used in async operations for the result channel.
	ResourcesOrErr struct {
		Resources []core.Resource
		Err       error
	}
)

func (c *Compiler) Compile() error {
	// Add our internal resource to be used for provider specific implementations. ex) aws dispatcher requires the payloads bucket and so does proxy
	// TODO: We could likely move this into runtime, but until we refactor that to be common we can keep this here so it lives in one place.
	// We previously always created the payloads bucket so the behavior is no different
	internalResource := &core.InternalResource{AnnotationKey: core.AnnotationKey{ID: core.KlothoPayloadName, Capability: annotation.InternalCapability}}
	c.Document.Constructs.AddConstruct(internalResource)

	for _, p := range c.AnalysisAndTransformationPlugins {
		if isPluginNil(p) {
			continue
		}
		log := zap.L().With(zap.String("plugin", p.Name()))
		log.Debug("starting")
		err := p.Transform(c.Document.InputFiles, c.Document.Constructs)
		if err != nil {
			return core.NewPluginError(p.Name(), err)
		}
		log.Debug("completed")
	}

	for _, p := range c.ProviderPlugins {
		if isPluginNil(p) {
			continue
		}
		log := zap.L().With(zap.String("plugin", p.Name()))
		log.Debug("starting")
		links, err := p.Translate(c.Document.Constructs, c.Document.Resources)
		if err != nil {
			return core.NewPluginError(p.Name(), err)
		}
		c.Document.Configuration.AddLinks(links)
		log.Debug("completed")
	}

	for _, p := range c.IaCPlugins {
		if isPluginNil(p) {
			continue
		}
		// TODO logging
		files, err := p.Translate(c.Document.Resources)
		if err != nil {
			return core.NewPluginError(p.Name(), err)
		}
		c.Document.OutputFiles = append(c.Document.OutputFiles, files...)
	}
	err := c.createConfigOutputFile()
	if err != nil {
		return errors.Wrap(err, "Unable to output Klotho configuration file")
	}
	return c.Document.OutputTo(c.Document.Configuration.OutDir)
}

func (c *Compiler) createConfigOutputFile() error {
	c.Document.Configuration.UpdateForResources(c.Document.Constructs.ListConstructs())
	buf := new(bytes.Buffer)
	var err error
	switch c.Document.Configuration.Format {
	case "toml":
		enc := toml.NewEncoder(buf)
		enc.SetArraysMultiline(true)
		enc.SetIndentTables(true)
		err = enc.Encode(c.Document.Configuration)

	case "json":
		err = json.NewEncoder(buf).Encode(c.Document.Configuration)

	case "yaml":
		err = yaml.NewEncoder(buf).Encode(c.Document.Configuration)

	default:
		err = errors.Errorf("unsupported config format: %s", c.Document.Configuration.Format)
	}
	if err != nil {
		return err
	}
	c.Document.OutputFiles = append(c.Document.OutputFiles, &core.RawFile{
		FPath:   fmt.Sprintf("klotho.%s", c.Document.Configuration.Format),
		Content: buf.Bytes(),
	})
	return nil
}

func isPluginNil(i Plugin) bool {
	if i == nil {
		return true
	}
	switch reflect.TypeOf(i).Kind() {
	case reflect.Pointer:
		return reflect.ValueOf(i).IsNil()
	}
	return false
}
