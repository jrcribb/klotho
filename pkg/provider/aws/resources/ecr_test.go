package resources

import (
	"testing"

	"github.com/klothoplatform/klotho/pkg/annotation"
	"github.com/klothoplatform/klotho/pkg/core"
	"github.com/klothoplatform/klotho/pkg/core/coretesting"
	"github.com/stretchr/testify/assert"
)

func Test_EcrRepositoryCreate(t *testing.T) {
	initialRefs := core.AnnotationKeySetOf(core.AnnotationKey{ID: "first"})
	cases := []struct {
		name string
		repo *EcrRepository
		want coretesting.ResourcesExpectation
	}{
		{
			name: "nil repo",
			want: coretesting.ResourcesExpectation{
				Nodes: []string{
					"aws:ecr_repo:my-app",
				},
				Deps: []coretesting.StringDep{},
			},
		},
		{
			name: "existing repo",
			repo: &EcrRepository{Name: "my-app", ConstructsRef: initialRefs, ForceDelete: true},
			want: coretesting.ResourcesExpectation{
				Nodes: []string{
					"aws:ecr_repo:my-app",
				},
				Deps: []coretesting.StringDep{},
			},
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			assert := assert.New(t)
			dag := core.NewResourceGraph()
			if tt.repo != nil {
				dag.AddResource(tt.repo)
			}
			metadata := RepoCreateParams{
				AppName: "my-app",
				Refs:    core.AnnotationKeySetOf(core.AnnotationKey{ID: "test", Capability: annotation.ExecutionUnitCapability}),
			}

			repo := &EcrRepository{}
			err := repo.Create(dag, metadata)

			if !assert.NoError(err) {
				return
			}

			tt.want.Assert(t, dag)
			graphRepo := dag.GetResource(repo.Id())
			repo = graphRepo.(*EcrRepository)
			assert.Equal(repo.Name, "my-app")
			if tt.repo == nil {
				assert.Equal(repo.ConstructsRef, metadata.Refs)
			} else {
				assert.Equal(repo, tt.repo)
				expect := initialRefs.CloneWith(core.AnnotationKeySetOf(core.AnnotationKey{ID: "test", Capability: annotation.ExecutionUnitCapability}))
				assert.Equal(repo.KlothoConstructRef(), expect)
			}
		})
	}
}

func Test_EcrImageCreate(t *testing.T) {
	initialRefs := core.AnnotationKeySetOf(core.AnnotationKey{ID: "first"})
	cases := []struct {
		name    string
		image   *EcrImage
		want    coretesting.ResourcesExpectation
		wantErr bool
	}{
		{
			name: "nil repo",
			want: coretesting.ResourcesExpectation{
				Nodes: []string{
					"aws:ecr_image:my-app-test-unit",
					"aws:ecr_repo:my-app",
				},
				Deps: []coretesting.StringDep{
					{Source: "aws:ecr_image:my-app-test-unit", Destination: "aws:ecr_repo:my-app"},
				},
			},
		},
		{
			name:    "existing image",
			image:   &EcrImage{Name: "my-app-test-unit", ConstructsRef: initialRefs},
			wantErr: true,
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			assert := assert.New(t)
			dag := core.NewResourceGraph()
			if tt.image != nil {
				dag.AddResource(tt.image)
			}
			metadata := ImageCreateParams{
				AppName: "my-app",
				Refs:    core.AnnotationKeySetOf(core.AnnotationKey{ID: "test", Capability: annotation.ExecutionUnitCapability}),
				Name:    "test-unit",
			}
			image := &EcrImage{}
			err := image.Create(dag, metadata)

			if tt.wantErr {
				assert.Error(err)
				return
			}
			if !assert.NoError(err) {
				return
			}
			tt.want.Assert(t, dag)
			graphImage := dag.GetResource(image.Id())
			image = graphImage.(*EcrImage)

			assert.Equal(image.Name, "my-app-test-unit")
			assert.Equal(image.ConstructsRef, metadata.Refs)
		})
	}
}

func Test_EcrImageConfigure(t *testing.T) {
	cases := []struct {
		name    string
		params  EcrImageConfigureParams
		want    *EcrImage
		wantErr bool
	}{
		{
			name: "filled params",
			params: EcrImageConfigureParams{
				Context:    "context",
				Dockerfile: "dockerfile",
			},
			want: &EcrImage{Context: "context", Dockerfile: "dockerfile", ExtraOptions: []string{"--platform", "linux/amd64", "--quiet"}},
		},
		{
			name:    "no context",
			params:  EcrImageConfigureParams{Dockerfile: "dockerfile"},
			wantErr: true,
		},
		{
			name:    "no dockerfile",
			params:  EcrImageConfigureParams{Context: "dockerfile"},
			wantErr: true,
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			assert := assert.New(t)

			image := &EcrImage{}
			err := image.Configure(tt.params)

			if tt.wantErr {
				assert.Error(err)
				return
			}
			if !assert.NoError(err) {
				return
			}

			assert.Equal(tt.want, image)
		})
	}
}

func Test_GenerateExecUnitResources(t *testing.T) {
	appName := "test-app"
	unit := &core.ExecutionUnit{AnnotationKey: core.AnnotationKey{ID: "test", Capability: annotation.ExecutionUnitCapability}}
	repo := NewEcrRepository(appName, core.AnnotationKey{ID: "test", Capability: annotation.ExecutionUnitCapability})
	image := NewEcrImage(&core.ExecutionUnit{AnnotationKey: core.AnnotationKey{ID: "test", Capability: annotation.ExecutionUnitCapability}}, appName, repo)

	cases := []struct {
		name         string
		existingRepo *EcrRepository
		want         *EcrImage
		wantErr      bool
	}{
		{
			name: "generate nothing existing",
			want: image,
		},
		{
			name:         "ecr repo already exists",
			existingRepo: NewEcrRepository(appName, core.AnnotationKey{ID: "test2", Capability: annotation.ExecutionUnitCapability}),
			want:         image,
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			assert := assert.New(t)
			dag := core.NewResourceGraph()

			if tt.existingRepo != nil {
				dag.AddResource(tt.existingRepo)

			}

			actualImage, err := GenerateEcrRepoAndImage(appName, unit, dag)
			if tt.wantErr {
				assert.Error(err)
				return
			}
			if !assert.NoError(err) {
				return
			}
			assert.Equal(tt.want.Name, actualImage.Name)
			assert.Equal(tt.want.ConstructsRef, actualImage.ConstructsRef)
			assert.Equal(tt.want.Context, actualImage.Context)
			assert.Equal(tt.want.Dockerfile, actualImage.Dockerfile)
			assert.Equal(tt.want.Repo.Id(), actualImage.Repo.Id())
			assert.Equal(tt.want.ExtraOptions, actualImage.ExtraOptions)

			for _, res := range dag.ListResources() {
				if repo, ok := res.(*EcrRepository); ok {
					if tt.existingRepo != nil {
						assert.Len(repo.ConstructsRef, 2)
					} else {
						assert.Len(repo.ConstructsRef, 1)
					}
				}
			}
		})

	}
}
