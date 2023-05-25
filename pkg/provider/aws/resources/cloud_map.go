package resources

import (
	"github.com/klothoplatform/klotho/pkg/core"
	"github.com/klothoplatform/klotho/pkg/sanitization/aws"
)

var privateDnsNamespaceSanitizer = aws.PrivateDnsNamespaceSanitizer

const (
	PRIVATE_DNS_NAMESPACE_TYPE = "private_dns_namespace"
)

type (
	PrivateDnsNamespace struct {
		Name          string
		ConstructsRef core.AnnotationKeySet
		Vpc           *Vpc
	}
)

func NewPrivateDnsNamespace(appName string, refs core.AnnotationKeySet, vpc *Vpc) *PrivateDnsNamespace {
	return &PrivateDnsNamespace{
		Name:          privateDnsNamespaceSanitizer.Apply(appName),
		ConstructsRef: refs,
		Vpc:           vpc,
	}
}

// KlothoConstructRef returns AnnotationKey of the klotho resource the cloud resource is correlated to
func (ns *PrivateDnsNamespace) KlothoConstructRef() core.AnnotationKeySet {
	return ns.ConstructsRef
}

// Id returns the id of the cloud resource
func (ns *PrivateDnsNamespace) Id() core.ResourceId {
	return core.ResourceId{
		Provider: AWS_PROVIDER,
		Type:     PRIVATE_DNS_NAMESPACE_TYPE,
		Name:     ns.Name,
	}
}
