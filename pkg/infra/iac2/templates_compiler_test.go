package iac2

import (
	"bytes"
	"fmt"
	"io/fs"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/klothoplatform/klotho/pkg/core"
	"github.com/klothoplatform/klotho/pkg/provider/aws/resources"
	"github.com/stretchr/testify/assert"
)

func TestOutputBody(t *testing.T) {
	fizz := &DummyFizz{Value: "my-hello"}
	buzz := DummyBuzz{}
	parent := &DummyBig{
		id:        "main",
		Fizz:      fizz,
		Buzz:      buzz,
		NestedDoc: &NestedResource{Fizz: fizz},
		NestedTemplate: &NestedTemplate{
			Str: "strVal",
			Arr: []string{"val1", "val2"},
		},
	}
	graph := core.NewResourceGraph()
	graph.AddResource(fizz)
	graph.AddResource(buzz)
	graph.AddResource(parent)
	graph.AddDependency2(parent, fizz)
	graph.AddDependency2(parent, buzz)
	graph.AddDependency2(fizz, buzz)

	compiler := CreateTemplatesCompiler(graph)
	compiler.templates = filesMapToFsMap(dummyTemplateFiles)

	t.Run("body", func(t *testing.T) {
		assert := assert.New(t)
		buf := bytes.Buffer{}
		err := compiler.RenderBody(&buf)
		if !assert.NoError(err) {
			return
		}
		expect := s(
			"const buzzShared = new aws.buzz.DummyResource();",
			"",
			"const fizzMyHello = new aws.fizz.DummyResource(`my-hello`);",
			"",
			"const bigMain = new DummyParent(",
			"				fizzMyHello,",
			"				{",
			"					buzz: buzzShared,",
			"					nestedDoc: {Fizz: fizzMyHello,",
			"}",
			"					nestedTemplate: ",
			"		{",
			"			str: \"strVal\"",
			"			arr0: \"val1\"",
			"			arr1: \"val2\"",
			"		}",
			"					rawNestedTemplate: true",
			"				});",
		)
		assert.Equal(expect, buf.String())
	})
	t.Run("imports", func(t *testing.T) {
		assert := assert.New(t)
		buf := bytes.Buffer{}
		err := compiler.RenderImports(&buf)
		if !assert.NoError(err) {
			return
		}
		expect := strings.TrimLeft(`
import * as aws from '@pulumi/aws'
import * as inputs from '@pulumi/aws/types/input'
import {Whatever} from "@pulumi/aws/cool/service"
`, "\n")
		assert.Equal(expect, buf.String())
	})
}

func TestResolveStructInput(t *testing.T) {
	cases := []struct {
		name                   string
		parentResource         *core.Resource
		value                  any
		withVars               map[string]string
		useDoubleQuotedStrings bool
		want                   string
	}{
		{
			name:  "string",
			value: "hello, world",
			want:  "`hello, world`",
		},
		{
			name:  "bool",
			value: true,
			want:  `true`,
		},
		{
			name:  "int",
			value: 123,
			want:  `123`,
		},
		{
			name:  "float",
			value: 1234.5,
			want:  `1234.5`,
		},
		{
			name:     "struct",
			value:    DummyBuzz{},
			withVars: map[string]string{`buzz-shared`: `myVar`},
			want:     `myVar`,
		},
		{
			name:     "struct-pointer",
			value:    &DummyFizz{Value: `abc`},
			withVars: map[string]string{`fizz-abc`: `myVar`},
			want:     `myVar`,
		},
		{
			name:  "null",
			value: nil,
			want:  `null`,
		},
		{
			name:     "slice of resources",
			value:    []core.Resource{&DummyFizz{Value: `abc`}},
			withVars: map[string]string{`fizz-abc`: `myVar`},
			want:     `[myVar]`,
		},
		{
			name:     "slice of any",
			value:    []any{123, &DummyFizz{Value: `abc`}},
			withVars: map[string]string{`fizz-abc`: `myVar`},
			want:     `[123,myVar]`,
		},
		{
			name:     "array",
			value:    [2]any{123, &DummyFizz{Value: `abc`}},
			withVars: map[string]string{`fizz-abc`: `myVar`},
			want:     `[123,myVar]`,
		},
		{
			name: "map",
			value: map[string]any{
				"MyStruct": DummyBuzz{},
			},
			withVars:               map[string]string{`buzz-shared`: `myVar`},
			want:                   `{"MyStruct":myVar}`,
			useDoubleQuotedStrings: true,
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			assert := assert.New(t)
			tc := TemplatesCompiler{
				resourceVarNamesById: tt.withVars,
			}
			resourceVal := reflect.ValueOf(tt.parentResource)
			val := reflect.ValueOf(tt.value)
			actual, err := tc.resolveStructInput(&resourceVal, val, tt.useDoubleQuotedStrings, "", nil)
			assert.NoError(err)
			assert.Equal(tt.want, actual)
		})
	}
}

func Test_handleIaCValue(t *testing.T) {
	cases := []struct {
		name                 string
		value                core.IaCValue
		resourceVarNamesById map[string]string
		want                 string
		wantOutputs          []AppliedOutput
	}{
		{
			name: "bucket name",
			value: core.IaCValue{
				Resource: resources.NewS3Bucket(&core.Fs{}, "test-app"),
				Property: string(core.BUCKET_NAME),
			},
			resourceVarNamesById: map[string]string{
				"aws:s3_bucket:test-app-": "testBucket",
			},
			want: "testBucket.bucket",
		},
		{
			name: "string value, nil resource",
			value: core.IaCValue{
				Property: "TestValue",
			},
			want: "`TestValue`",
		},
		{
			name: "value with applied outputs, cluster oidc arn",
			value: core.IaCValue{
				Resource: resources.NewEksCluster("test-app", "cluster1", nil, nil, nil),
				Property: resources.CLUSTER_OIDC_ARN_IAC_VALUE,
			},
			resourceVarNamesById: map[string]string{
				"aws:eks_cluster:test-app-cluster1": "awsEksClusterTestAppCluster1",
			},
			want: "`arn:aws:iam::${cluster_arn.split(':')[4]}:oidc-provider/${cluster_oidc_url}`",
			wantOutputs: []AppliedOutput{
				{
					appliedName: fmt.Sprintf("%s.openIdConnectIssuerUrl", "awsEksClusterTestAppCluster1"),
					varName:     "cluster_oidc_url",
				},
				{
					appliedName: fmt.Sprintf("%s.arn", "awsEksClusterTestAppCluster1"),
					varName:     "cluster_arn",
				},
			},
		},
		{
			name: "Availability zone",
			value: core.IaCValue{
				Resource: &resources.AvailabilityZones{},
				Property: "2",
			},
			resourceVarNamesById: map[string]string{
				"aws:availability_zones:AvailabilityZones": "azs",
			},
			want: "awsAvailabilityZones.names[2]",
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			assert := assert.New(t)
			tc := TemplatesCompiler{
				resourceVarNames:     map[string]struct{}{},
				resourceVarNamesById: tt.resourceVarNamesById,
			}
			appliedOutputs := []AppliedOutput{}
			actual, err := tc.handleIaCValue(tt.value, &appliedOutputs)
			assert.NoError(err)
			assert.Equal(tt.want, actual)
			if tt.wantOutputs != nil {
				assert.ElementsMatch(tt.wantOutputs, appliedOutputs)
			}
		})
	}
}

type (
	DummyFizz struct {
		Value string
	}

	DummyBuzz struct {
		// nothing
	}

	DummyBig struct {
		id             string
		Fizz           *DummyFizz
		Buzz           DummyBuzz
		NestedDoc      *NestedResource `render:"document"`
		NestedTemplate *NestedTemplate `render:"template"`
	}

	NestedResource struct {
		Fizz *DummyFizz
	}

	NestedTemplate struct {
		Str string
		Arr []string
	}
)

func (f *DummyFizz) Id() string                               { return "fizz-" + f.Value }
func (f *DummyFizz) Provider() string                         { return "DummyProvider" }
func (f *DummyFizz) KlothoConstructRef() []core.AnnotationKey { return nil }

func (b DummyBuzz) Id() string                               { return "buzz-shared" }
func (f DummyBuzz) Provider() string                         { return "DummyProvider" }
func (f DummyBuzz) KlothoConstructRef() []core.AnnotationKey { return nil }

func (p *DummyBig) Id() string                               { return "big-" + p.id }
func (f *DummyBig) Provider() string                         { return "DummyProvider" }
func (f *DummyBig) KlothoConstructRef() []core.AnnotationKey { return nil }

var dummyTemplateFiles = map[string]string{
	`dummy_fizz/factory.ts`: `
		import * as aws from '@pulumi/aws'

		interface Args {
			Value: string,
		}

		function create(args: Args): aws.fizz.DummyResource {
			return new aws.fizz.DummyResource(args.Value);
		}`,

	`dummy_buzz/factory.ts`: `
		import * as aws from '@pulumi/aws'
		import {Whatever} from "@pulumi/aws/cool/service"; // Note the trailing semicolon. It'll get removed.

		interface Args {}

		function create(args: Args): aws.buzz.DummyResource {
			return new aws.buzz.DummyResource();
		}`,

	`dummy_big/nested_template.ts.tmpl`: `
		{
			str: "{{.Str}}"
			{{- range $index, $val := .Arr }}
			arr{{$index}}: "{{$val}}"
			{{- end}}
		}`,
	`dummy_big/factory.ts`: `
		import * as aws from '@pulumi/aws'
		import * as inputs from '@pulumi/aws/types/input'

		interface Args {
			Fizz: aws.fizz.DummyResource,
			Buzz: aws.buzz.DummyResource,
			NestedDoc: aws.nest.DummyResource
			NestedTemplate: pulumi.Input<inputs.nest.NestedInput>
		}

		function create(args: Args): aws.foobar.DummyParent {
			return new DummyParent(
				args.Fizz,
				{
					buzz: args.Buzz,
					nestedDoc: args.NestedDoc
					nestedTemplate: args.NestedTemplate
					//TMPL {{- if eq .NestedTemplate.Raw.Str "strVal"}}
					rawNestedTemplate: true
					//TMPL {{- end}}
				});
		}`,
}

func filesMapToFsMap(files map[string]string) fs.FS {
	mockFs := make(fstest.MapFS)
	for path, contents := range files {
		mockFs[path] = &fstest.MapFile{
			Data:    []byte(contents),
			Mode:    0700,
			ModTime: time.Now(),
			Sys:     struct{}{},
		}
	}
	return mockFs
}

// s joins all the inputs via newline. We use it because the output might itself contain backticks, which makes the
// built-in go multiline strings unuseful.
func s(lines ...string) string {
	return strings.Join(lines, "\n")
}
