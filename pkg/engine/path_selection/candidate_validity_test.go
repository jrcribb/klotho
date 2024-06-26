package path_selection

import (
	"testing"

	"github.com/klothoplatform/klotho/pkg/construct"
	"github.com/klothoplatform/klotho/pkg/construct/graphtest"
	"github.com/klothoplatform/klotho/pkg/engine/enginetesting"
	knowledgebase "github.com/klothoplatform/klotho/pkg/knowledgebase"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func Test_checkModifiesImportedResource(t *testing.T) {
	tests := []struct {
		name   string
		source *construct.Resource
		target *construct.Resource
		et     *knowledgebase.EdgeTemplate
		mocks  func(*enginetesting.MockKB)
		want   bool
	}{
		{
			name:   "no imported resource returns true",
			source: &construct.Resource{ID: graphtest.ParseId(t, "p:a:a")},
			target: &construct.Resource{ID: graphtest.ParseId(t, "p:b:b")},
			et:     &knowledgebase.EdgeTemplate{},
			want:   false,
		},
		{
			name:   "imported resource with no modifications returns true",
			source: &construct.Resource{ID: graphtest.ParseId(t, "p:a:a"), Imported: true},
			target: &construct.Resource{ID: graphtest.ParseId(t, "p:b:b")},
			et: &knowledgebase.EdgeTemplate{
				OperationalRules: []knowledgebase.OperationalRule{
					{
						ConfigurationRules: []knowledgebase.ConfigurationRule{
							{
								Resource: "{{ .Target }}",
							},
						},
					},
				},
			},
			want: false,
		},
		{
			name:   "imported resource with modifications returns false",
			source: &construct.Resource{ID: graphtest.ParseId(t, "p:a:a"), Imported: true},
			target: &construct.Resource{ID: graphtest.ParseId(t, "p:b:b")},
			et: &knowledgebase.EdgeTemplate{
				OperationalRules: []knowledgebase.OperationalRule{
					{
						ConfigurationRules: []knowledgebase.ConfigurationRule{
							{
								Resource: "{{ .Source }}",
							},
						},
					},
				},
			},
			want: true,
		},
		{
			name:   "gets edge template if not provided",
			source: &construct.Resource{ID: graphtest.ParseId(t, "p:a:a"), Imported: true},
			target: &construct.Resource{ID: graphtest.ParseId(t, "p:b:b")},
			mocks: func(kb *enginetesting.MockKB) {
				kb.On("GetEdgeTemplate", graphtest.ParseId(t, "p:a:a"), graphtest.ParseId(t, "p:b:b")).Return(&knowledgebase.EdgeTemplate{})
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sol := enginetesting.NewTestSolution()
			sol.KB.On("GetResourceTemplate", mock.Anything).Return(&knowledgebase.ResourceTemplate{}, nil)
			if tt.mocks != nil {
				tt.mocks(&sol.KB)
			}
			err := sol.RawView().AddVertex(tt.source)
			require.NoError(t, err)
			err = sol.RawView().AddVertex(tt.target)
			require.NoError(t, err)

			got, err := checkModifiesImportedResource(tt.source.ID, tt.target.ID, sol, tt.et)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
			sol.KB.AssertExpectations(t)
		})
	}
}

func Test_checkAsTargetValidity(t *testing.T) {
	type testResource struct {
		id    string
		props map[string]string
	}
	type args struct {
		graph          []any
		kb             func(t *testing.T, kb *enginetesting.MockKB)
		resource       testResource
		source         string
		classification string
	}
	tests := []struct {
		name    string
		args    args
		want    bool
		wantErr bool
	}{
		{
			name: "no path satisfaction rules",
			args: args{
				graph: []any{"p:a:a -> p:c:c"},
				kb: func(t *testing.T, kb *enginetesting.MockKB) {
					matches := func(id string) any {
						return mock.MatchedBy(graphtest.ParseId(t, id).Matches)
					}
					kb.On("GetResourceTemplate", matches("p:a")).Return(&knowledgebase.ResourceTemplate{}, nil)
					kb.On("GetResourceTemplate", matches("p:c")).Return(&knowledgebase.ResourceTemplate{}, nil)
					kb.On("GetEdgeTemplate", matches("p:a"), matches("p:c")).Return(&knowledgebase.EdgeTemplate{})
				},
				resource:       testResource{id: "p:c:c"},
				source:         "p:a:a",
				classification: "network",
			},
			want: true,
		},
		{
			name: "simple as target matches",
			args: args{
				graph: []any{"p:a:a -> p:c:c"},
				kb: func(t *testing.T, kb *enginetesting.MockKB) {
					matches := func(id string) any {
						return mock.MatchedBy(graphtest.ParseId(t, id).Matches)
					}
					kb.On("GetResourceTemplate", matches("p:a")).Return(&knowledgebase.ResourceTemplate{}, nil)
					kb.On("GetResourceTemplate", matches("p:c")).Return(&knowledgebase.ResourceTemplate{
						PathSatisfaction: knowledgebase.PathSatisfaction{
							AsTarget: []knowledgebase.PathSatisfactionRoute{
								{
									Classification: "network",
									Validity:       knowledgebase.DownstreamOperation,
								},
							},
						},
					}, nil)
					kb.On("GetEdgeTemplate", matches("p:a"), matches("p:c")).Return(&knowledgebase.EdgeTemplate{})
				},
				resource:       testResource{id: "p:c:c"},
				source:         "p:a:a",
				classification: "network",
			},
			want: true,
		},
		{
			name: "target does not match",
			args: args{
				graph: []any{"p:a:a", "p:c:c"},
				kb: func(t *testing.T, kb *enginetesting.MockKB) {
					matches := func(id string) any {
						return mock.MatchedBy(graphtest.ParseId(t, id).Matches)
					}
					kb.On("GetResourceTemplate", matches("p:a")).Return(&knowledgebase.ResourceTemplate{}, nil)
					kb.On("GetResourceTemplate", matches("p:c")).Return(&knowledgebase.ResourceTemplate{
						PathSatisfaction: knowledgebase.PathSatisfaction{
							AsTarget: []knowledgebase.PathSatisfactionRoute{
								{
									Classification: "network",
									Validity:       knowledgebase.DownstreamOperation,
								},
							},
						},
					}, nil)
					kb.On("GetEdgeTemplate", matches("p:a"), matches("p:c")).Return(&knowledgebase.EdgeTemplate{})
				},
				resource:       testResource{id: "p:c:c"},
				source:         "p:a:a",
				classification: "network",
			},
			want: false,
		},
		{
			name: "property ref no value",
			args: args{
				graph: []any{"p:a:a -> p:c:c"},
				kb: func(t *testing.T, kb *enginetesting.MockKB) {
					matches := func(id string) any {
						return mock.MatchedBy(graphtest.ParseId(t, id).Matches)
					}
					kb.On("GetResourceTemplate", matches("p:a")).Return(&knowledgebase.ResourceTemplate{}, nil)
					kb.On("GetResourceTemplate", matches("p:c")).Return(&knowledgebase.ResourceTemplate{
						PathSatisfaction: knowledgebase.PathSatisfaction{
							AsTarget: []knowledgebase.PathSatisfactionRoute{
								{
									Classification:    "network",
									PropertyReference: "X",
									Validity:          knowledgebase.DownstreamOperation,
								},
							},
						},
					}, nil)
					kb.On("GetEdgeTemplate", matches("p:a"), matches("p:c")).Return(&knowledgebase.EdgeTemplate{})
				},
				resource:       testResource{id: "p:c:c"},
				source:         "p:a:a",
				classification: "network",
			},
			want: true,
		},
		{
			name: "property ref valid value",
			args: args{
				graph: []any{"p:a:a -> p:c:c", "p:a:a -> p:x:x"},
				kb: func(t *testing.T, kb *enginetesting.MockKB) {
					matches := func(id string) any {
						return mock.MatchedBy(graphtest.ParseId(t, id).Matches)
					}
					kb.On("GetResourceTemplate", matches("p:a")).Return(&knowledgebase.ResourceTemplate{}, nil)
					kb.On("GetResourceTemplate", matches("p:c")).Return(&knowledgebase.ResourceTemplate{
						PathSatisfaction: knowledgebase.PathSatisfaction{
							AsTarget: []knowledgebase.PathSatisfactionRoute{
								{
									Classification:    "network",
									PropertyReference: "X",
									Validity:          knowledgebase.DownstreamOperation,
								},
							},
						},
					}, nil)
					kb.On("GetResourceTemplate", matches("p:x")).Return(&knowledgebase.ResourceTemplate{}, nil)
					kb.On("GetEdgeTemplate", matches("p:a"), matches("p:c")).Return(&knowledgebase.EdgeTemplate{})
					kb.On("GetEdgeTemplate", matches("p:a"), matches("p:x")).Return(&knowledgebase.EdgeTemplate{})
				},
				resource:       testResource{id: "p:c:c", props: map[string]string{"X": "p:x:x"}},
				source:         "p:a:a",
				classification: "network",
			},
			want: true,
		},
		{
			name: "property ref invalid value",
			args: args{
				graph: []any{"p:a:a -> p:c:c", "p:x:x"},
				kb: func(t *testing.T, kb *enginetesting.MockKB) {
					matches := func(id string) any {
						return mock.MatchedBy(graphtest.ParseId(t, id).Matches)
					}
					kb.On("GetResourceTemplate", matches("p:a")).Return(&knowledgebase.ResourceTemplate{}, nil)
					kb.On("GetResourceTemplate", matches("p:c")).Return(&knowledgebase.ResourceTemplate{
						PathSatisfaction: knowledgebase.PathSatisfaction{
							AsTarget: []knowledgebase.PathSatisfactionRoute{
								{
									Classification:    "network",
									PropertyReference: "X",
									Validity:          knowledgebase.DownstreamOperation,
								},
							},
						},
					}, nil)
					kb.On("GetResourceTemplate", matches("p:x")).Return(&knowledgebase.ResourceTemplate{}, nil)
					kb.On("GetEdgeTemplate", matches("p:a"), matches("p:c")).Return(&knowledgebase.EdgeTemplate{})
				},
				resource:       testResource{id: "p:c:c", props: map[string]string{"X": "p:x:x"}},
				source:         "p:a:a",
				classification: "network",
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sol := enginetesting.NewTestSolution()
			sol.KB.Test(t)
			tt.args.kb(t, &sol.KB)
			sol.LoadState(t, tt.args.graph...)

			res, err := sol.RawView().Vertex(graphtest.ParseId(t, tt.args.resource.id))
			require.NoError(t, err)
			for k, v := range tt.args.resource.props {
				require.NoError(t, res.SetProperty(k, graphtest.ParseId(t, v)))
			}

			src := graphtest.ParseId(t, tt.args.source)

			got, err := checkAsTargetValidity(sol, res, src, tt.args.classification)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func Test_checkAsSourceValidity(t *testing.T) {
	type testResource struct {
		id    string
		props map[string]string
	}
	type args struct {
		graph          []any
		kb             func(t *testing.T, kb *enginetesting.MockKB)
		resource       testResource
		source         string
		classification string
	}
	tests := []struct {
		name    string
		args    args
		want    bool
		wantErr bool
	}{
		{
			name: "no path satisfaction rules",
			args: args{
				graph: []any{"p:a:a -> p:c:c"},
				kb: func(t *testing.T, kb *enginetesting.MockKB) {
					matches := func(id string) any {
						return mock.MatchedBy(graphtest.ParseId(t, id).Matches)
					}
					kb.On("GetResourceTemplate", matches("p:a")).Return(&knowledgebase.ResourceTemplate{}, nil)
					kb.On("GetResourceTemplate", matches("p:c")).Return(&knowledgebase.ResourceTemplate{}, nil)
					kb.On("GetEdgeTemplate", matches("p:a"), matches("p:c")).Return(&knowledgebase.EdgeTemplate{})
				},
				resource:       testResource{id: "p:c:c"},
				source:         "p:a:a",
				classification: "network",
			},
			want: true,
		},
		{
			name: "simple as source matches",
			args: args{
				graph: []any{"p:a:a -> p:c:c"},
				kb: func(t *testing.T, kb *enginetesting.MockKB) {
					matches := func(id string) any {
						return mock.MatchedBy(graphtest.ParseId(t, id).Matches)
					}
					kb.On("GetResourceTemplate", matches("p:a")).Return(&knowledgebase.ResourceTemplate{
						PathSatisfaction: knowledgebase.PathSatisfaction{
							AsSource: []knowledgebase.PathSatisfactionRoute{
								{
									Classification: "network",
									Validity:       knowledgebase.DownstreamOperation,
								},
							},
						},
					}, nil)
					kb.On("GetResourceTemplate", matches("p:c")).Return(&knowledgebase.ResourceTemplate{}, nil)
					kb.On("GetEdgeTemplate", matches("p:a"), matches("p:c")).Return(&knowledgebase.EdgeTemplate{})
				},
				resource:       testResource{id: "p:a:a"},
				source:         "p:c:c",
				classification: "network",
			},
			want: true,
		},
		{
			name: "source does not match",
			args: args{
				graph: []any{"p:a:a", "p:c:c"},
				kb: func(t *testing.T, kb *enginetesting.MockKB) {
					matches := func(id string) any {
						return mock.MatchedBy(graphtest.ParseId(t, id).Matches)
					}
					kb.On("GetResourceTemplate", matches("p:a")).Return(&knowledgebase.ResourceTemplate{
						PathSatisfaction: knowledgebase.PathSatisfaction{
							AsSource: []knowledgebase.PathSatisfactionRoute{
								{
									Classification: "network",
									Validity:       knowledgebase.DownstreamOperation,
								},
							},
						},
					}, nil)
					kb.On("GetResourceTemplate", matches("p:c")).Return(&knowledgebase.ResourceTemplate{}, nil)
					kb.On("GetEdgeTemplate", matches("p:a"), matches("p:c")).Return(&knowledgebase.EdgeTemplate{})
				},
				resource:       testResource{id: "p:a:a"},
				source:         "p:c:c",
				classification: "network",
			},
			want: false,
		},
		{
			name: "property ref no value",
			args: args{
				graph: []any{"p:a:a -> p:c:c"},
				kb: func(t *testing.T, kb *enginetesting.MockKB) {
					matches := func(id string) any {
						return mock.MatchedBy(graphtest.ParseId(t, id).Matches)
					}
					kb.On("GetResourceTemplate", matches("p:a")).Return(&knowledgebase.ResourceTemplate{
						PathSatisfaction: knowledgebase.PathSatisfaction{
							AsSource: []knowledgebase.PathSatisfactionRoute{
								{
									Classification:    "network",
									PropertyReference: "X",
									Validity:          knowledgebase.DownstreamOperation,
								},
							},
						},
					}, nil)
					kb.On("GetResourceTemplate", matches("p:c")).Return(&knowledgebase.ResourceTemplate{}, nil)
					kb.On("GetEdgeTemplate", matches("p:a"), matches("p:c")).Return(&knowledgebase.EdgeTemplate{})
				},
				resource:       testResource{id: "p:a:a"},
				source:         "p:c:c",
				classification: "network",
			},
			want: true,
		},
		{
			name: "property ref valid value",
			args: args{
				graph: []any{"p:a:a -> p:c:c", "p:x:x -> p:c:c"},
				kb: func(t *testing.T, kb *enginetesting.MockKB) {
					matches := func(id string) any {
						return mock.MatchedBy(graphtest.ParseId(t, id).Matches)
					}
					kb.On("GetResourceTemplate", matches("p:a")).Return(&knowledgebase.ResourceTemplate{
						PathSatisfaction: knowledgebase.PathSatisfaction{
							AsSource: []knowledgebase.PathSatisfactionRoute{
								{
									Classification:    "network",
									PropertyReference: "X",
									Validity:          knowledgebase.DownstreamOperation,
								},
							},
						},
					}, nil)
					kb.On("GetResourceTemplate", matches("p:c")).Return(&knowledgebase.ResourceTemplate{}, nil)
					kb.On("GetResourceTemplate", matches("p:x")).Return(&knowledgebase.ResourceTemplate{}, nil)
					kb.On("GetEdgeTemplate", matches("p:a"), matches("p:c")).Return(&knowledgebase.EdgeTemplate{})
					kb.On("GetEdgeTemplate", matches("p:x"), matches("p:c")).Return(&knowledgebase.EdgeTemplate{})
				},
				resource:       testResource{id: "p:a:a", props: map[string]string{"X": "p:x:x"}},
				source:         "p:c:c",
				classification: "network",
			},
			want: true,
		},
		{
			name: "property ref invalid value",
			args: args{
				graph: []any{"p:a:a -> p:c:c", "p:x:x"},
				kb: func(t *testing.T, kb *enginetesting.MockKB) {
					matches := func(id string) any {
						return mock.MatchedBy(graphtest.ParseId(t, id).Matches)
					}
					kb.On("GetResourceTemplate", matches("p:a")).Return(&knowledgebase.ResourceTemplate{
						PathSatisfaction: knowledgebase.PathSatisfaction{
							AsSource: []knowledgebase.PathSatisfactionRoute{
								{
									Classification:    "network",
									PropertyReference: "X",
									Validity:          knowledgebase.DownstreamOperation,
								},
							},
						},
					}, nil)
					kb.On("GetResourceTemplate", matches("p:c")).Return(&knowledgebase.ResourceTemplate{}, nil)
					kb.On("GetResourceTemplate", matches("p:x")).Return(&knowledgebase.ResourceTemplate{}, nil)
					kb.On("GetEdgeTemplate", matches("p:a"), matches("p:c")).Return(&knowledgebase.EdgeTemplate{})
				},
				resource:       testResource{id: "p:a:a", props: map[string]string{"X": "p:x:x"}},
				source:         "p:c:c",
				classification: "network",
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sol := enginetesting.NewTestSolution()
			sol.KB.Test(t)
			tt.args.kb(t, &sol.KB)
			sol.LoadState(t, tt.args.graph...)

			res, err := sol.RawView().Vertex(graphtest.ParseId(t, tt.args.resource.id))
			require.NoError(t, err)
			for k, v := range tt.args.resource.props {
				require.NoError(t, res.SetProperty(k, graphtest.ParseId(t, v)))
			}

			src := graphtest.ParseId(t, tt.args.source)

			got, err := checkAsSourceValidity(sol, res, src, tt.args.classification)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
