package stateconverter

import (
	"bytes"
	"testing"

	"github.com/klothoplatform/klotho/pkg/construct"
	statetemplate "github.com/klothoplatform/klotho/pkg/infra/state_reader/state_template"
	"github.com/stretchr/testify/assert"
)

func Test_pulumiStateConverter_ConvertState(t *testing.T) {
	tests := []struct {
		name      string
		templates map[string]statetemplate.StateTemplate
		data      []byte
		want      State
	}{
		{
			name: "converts the state to the internal model",
			templates: map[string]statetemplate.StateTemplate{
				"aws:lambda/Function:Function": {
					QualifiedTypeName: "aws:lambda_function",
					IaCQualifiedType:  "aws:lambda/Function:Function",
					PropertyMappings: map[string]string{
						"arn": "Arn",
						"id":  "Id",
					},
				},
			},
			data: []byte(`[
				{
					"urn": "urn:my_lambda",
					"type": "aws:lambda/Function:Function",
					"outputs": {
						"arn": "arn",
						"id": "id"
					}
				}
			]`),
			want: State{
				construct.ResourceId{Provider: "aws", Type: "lambda_function", Name: "my_lambda"}: construct.Properties{
					"Arn": "arn",
					"Id":  "id",
				},
			},
		},
		{
			name:      "No mapping does not return error",
			templates: map[string]statetemplate.StateTemplate{},
			data: []byte(`[
				{
					"urn": "urn:my_lambda",
					"type": "aws:lambda/Function:Function",
					"outputs": {
						"arn": "arn",
						"id": "id"
					}
				}
			]`),
			want: State{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert := assert.New(t)
			p := pulumiStateConverter{
				templates: tt.templates,
			}
			reader := bytes.NewReader(tt.data)
			got, err := p.ConvertState(reader)
			if !assert.NoError(err) {
				return
			}
			assert.Equal(tt.want, got)
		})
	}
}

func Test_pulumiStateConverter_convertResource(t *testing.T) {
	tests := []struct {
		name     string
		resource Resource
		template statetemplate.StateTemplate
		want     construct.Resource
	}{
		{
			name: "converts the state to the internal model",
			template: statetemplate.StateTemplate{
				QualifiedTypeName: "aws:lambda_function",
				IaCQualifiedType:  "aws:lambda/Function:Function",
				PropertyMappings: map[string]string{
					"arn": "Arn",
					"id":  "Id",
				},
			},
			resource: Resource{
				Urn:  "urn:my_lambda",
				Type: "aws:lambda/Function:Function",
				Outputs: map[string]interface{}{
					"arn": "arn",
					"id":  "id",
				},
			},
			want: construct.Resource{
				ID: construct.ResourceId{Provider: "aws", Type: "lambda_function", Name: "my_lambda"},
				Properties: construct.Properties{
					"Arn": "arn",
					"Id":  "id",
				},
			},
		},
		{
			name: "reference fields are ignored",
			template: statetemplate.StateTemplate{
				QualifiedTypeName: "aws:lambda_function",
				IaCQualifiedType:  "aws:lambda/Function:Function",
				PropertyMappings: map[string]string{
					"arn":   "Arn",
					"id":    "Id",
					"vpcId": "Vpc#Id",
				},
			},
			resource: Resource{
				Urn:  "urn:my_lambda",
				Type: "aws:lambda/Function:Function",
				Outputs: map[string]interface{}{
					"arn":   "arn",
					"id":    "id",
					"vpcId": "vpc-1",
				},
			},
			want: construct.Resource{
				ID: construct.ResourceId{Provider: "aws", Type: "lambda_function", Name: "my_lambda"},
				Properties: construct.Properties{
					"Arn":    "arn",
					"Id":     "id",
					"Vpc#Id": "vpc-1",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert := assert.New(t)
			p := pulumiStateConverter{}
			got, err := p.convertResource(tt.resource, tt.template)
			if !assert.NoError(err) {
				return
			}
			assert.Equal(tt.want, *got)
		})
	}
}
