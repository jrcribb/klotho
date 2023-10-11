package operational_rule

import (
	"testing"

	construct "github.com/klothoplatform/klotho/pkg/construct2"
	"github.com/klothoplatform/klotho/pkg/engine2/enginetesting"
	knowledgebase "github.com/klothoplatform/klotho/pkg/knowledge_base2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func Test_SpreadPlacer(t *testing.T) {
	tests := []struct {
		name               string
		resource           *construct.Resource
		availableResources []*construct.Resource
		initialState       []any
		step               knowledgebase.OperationalStep
		numNeeded          int
		want               Result
	}{
		// {
		// 	name: "does nothing if no available resources",
		// 	resource: construct.CreateResource(construct.ResourceId{
		// 		Type: "test",
		// 		Name: "test1",
		// 	}),
		// 	numNeeded: 1,
		// },
		// {
		// 	name: "does nothing if one available resource",
		// 	resource: construct.CreateResource(construct.ResourceId{
		// 		Type: "test",
		// 		Name: "test1",
		// 	}),
		// 	availableResources: []*construct.Resource{
		// 		construct.CreateResource(construct.ResourceId{
		// 			Type: "test",
		// 			Name: "test2",
		// 		}),
		// 	},
		// 	numNeeded: 1,
		// },
		// {
		// 	name: "no resources placed yet, places in first resource",
		// 	resource: construct.CreateResource(construct.ResourceId{
		// 		Provider: "test",
		// 		Type:     "test",
		// 		Name:     "test1",
		// 	}),
		// 	availableResources: []*construct.Resource{
		// 		construct.CreateResource(construct.ResourceId{
		// 			Provider: "test",
		// 			Type:     "parent",
		// 			Name:     "test2",
		// 		}),
		// 		construct.CreateResource(construct.ResourceId{
		// 			Provider: "test",
		// 			Type:     "parent",
		// 			Name:     "test3",
		// 		}),
		// 	},
		// 	initialState: []any{
		// 		"test:test:test1",
		// 		"test:parent:test2",
		// 		"test:parent:test3",
		// 	},
		// 	step: knowledgebase.OperationalStep{
		// 		Direction: knowledgebase.DirectionDownstream,
		// 	},
		// 	numNeeded: 1,
		// 	want: Result{
		// 		AddedDependencies: []construct.Edge{
		// 			{
		// 				Source: construct.ResourceId{Provider: "test", Type: "test", Name: "test1"},
		// 				Target: construct.ResourceId{Provider: "test", Type: "parent", Name: "test2"},
		// 			},
		// 		},
		// 	},
		// },
		{
			name: "chooses placement in spot with least dependencies",
			resource: construct.CreateResource(construct.ResourceId{
				Provider: "test",
				Type:     "test",
				Name:     "test1",
			}),
			availableResources: []*construct.Resource{
				construct.CreateResource(construct.ResourceId{
					Provider: "test",
					Type:     "parent",
					Name:     "test2",
				}),
				construct.CreateResource(construct.ResourceId{
					Provider: "test",
					Type:     "parent",
					Name:     "test3",
				}),
			},
			initialState: []any{
				"test:test:test1",
				"test:test:test2",
				"test:parent:test2",
				"test:parent:test3",
				"test:test:test2 -> test:parent:test2",
			},
			step: knowledgebase.OperationalStep{
				Direction: knowledgebase.DirectionDownstream,
			},
			numNeeded: 1,
			want: Result{
				AddedDependencies: []construct.Edge{
					{
						Source: construct.ResourceId{Provider: "test", Type: "test", Name: "test1"},
						Target: construct.ResourceId{Provider: "test", Type: "parent", Name: "test3"},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert := assert.New(t)
			p := &SpreadPlacer{}
			testSol := enginetesting.NewTestSolution()
			testSol.KB.On("GetEdgeTemplate", mock.Anything, mock.Anything).Return(&knowledgebase.EdgeTemplate{})
			testSol.LoadState(t, tt.initialState...)
			p.SetCtx(OperationalRuleContext{
				Solution: testSol,
			})
			result, err := p.PlaceResources(tt.resource, tt.step, tt.availableResources, &tt.numNeeded)
			if err != nil {
				t.Errorf("PlaceResources() error = %v", err)
				return
			}
			assert.Equal(result, tt.want)
		})
	}

}
