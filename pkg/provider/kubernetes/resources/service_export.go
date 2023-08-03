package resources

import (
	"fmt"
	cloudmap "github.com/aws/aws-cloud-map-mcs-controller-for-k8s/pkg/apis/multicluster/v1alpha1"
	"github.com/klothoplatform/klotho/pkg/core"
	"github.com/klothoplatform/klotho/pkg/provider"
	"k8s.io/apimachinery/pkg/runtime"
)

type (
	ServiceExport struct {
		Name          string
		ConstructRefs core.BaseConstructSet `yaml:"-"`
		Object        *cloudmap.ServiceExport
		FilePath      string
		Cluster       core.ResourceId
	}
)

const (
	SERVICE_EXPORT_TYPE = "service_export"
)

func (se *ServiceExport) BaseConstructRefs() core.BaseConstructSet {
	return se.ConstructRefs
}

func (se *ServiceExport) Id() core.ResourceId {
	return core.ResourceId{
		Provider: provider.KUBERNETES,
		Type:     SERVICE_EXPORT_TYPE,
		Name:     se.Name,
	}
}

func (sa *ServiceExport) DeleteContext() core.DeleteContext {
	return core.DeleteContext{
		RequiresNoUpstream: true,
	}
}

func (se *ServiceExport) GetObject() runtime.Object {
	return se.Object
}

func (se *ServiceExport) Kind() string {
	return "ServiceExport"
}

func (se *ServiceExport) Path() string {
	return se.FilePath
}

func (se *ServiceExport) MakeOperational(dag *core.ResourceGraph, appName string) error {
	if se.Cluster.Name == "" {
		return fmt.Errorf("service export %s has no cluster", se.Name)
	}
	SetDefaultObjectMeta(se, se.Object.GetObjectMeta())
	se.FilePath = ManifestFilePath(se, se.Cluster)
	return nil
}
