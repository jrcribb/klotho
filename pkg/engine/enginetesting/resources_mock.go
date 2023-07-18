package enginetesting

import (
	"github.com/klothoplatform/klotho/pkg/core"
)

type (
	mockResource1 struct {
		Name          string
		ConstructRefs core.BaseConstructSet `yaml:"-"`
	}
	mockResource2 struct {
		Name          string
		ConstructRefs core.BaseConstructSet `yaml:"-"`
	}
	mockResource3 struct {
		Name          string
		ConstructRefs core.BaseConstructSet `yaml:"-"`
	}
	mockResource4 struct {
		Name          string
		ConstructRefs core.BaseConstructSet `yaml:"-"`
	}
)

func (f *mockResource1) Id() core.ResourceId {
	return core.ResourceId{Provider: "mock", Type: "mock1", Name: f.Name}
}
func (f *mockResource1) BaseConstructRefs() core.BaseConstructSet { return f.ConstructRefs }
func (f *mockResource1) DeleteContext() core.DeleteContext {
	return core.DeleteContext{
		RequiresNoUpstream:   true,
		RequiresNoDownstream: true,
	}
}
func (f *mockResource2) Id() core.ResourceId {
	return core.ResourceId{Provider: "mock", Type: "mock2", Name: f.Name}
}
func (f *mockResource2) BaseConstructRefs() core.BaseConstructSet { return f.ConstructRefs }
func (f *mockResource2) DeleteContext() core.DeleteContext {
	return core.DeleteContext{
		RequiresNoUpstream:   true,
		RequiresNoDownstream: true,
	}
}
func (f *mockResource3) Id() core.ResourceId {
	return core.ResourceId{Provider: "mock", Type: "mock3", Name: f.Name}
}
func (f *mockResource3) BaseConstructRefs() core.BaseConstructSet { return f.ConstructRefs }
func (f *mockResource3) DeleteContext() core.DeleteContext {
	return core.DeleteContext{
		RequiresNoUpstream:   true,
		RequiresNoDownstream: true,
	}
}

func (f *mockResource4) Id() core.ResourceId {
	return core.ResourceId{Provider: "mock", Type: "mock4", Name: f.Name}
}
func (f *mockResource4) BaseConstructRefs() core.BaseConstructSet { return f.ConstructRefs }
func (f *mockResource4) DeleteContext() core.DeleteContext {
	return core.DeleteContext{
		RequiresNoUpstream:   true,
		RequiresNoDownstream: true,
	}
}