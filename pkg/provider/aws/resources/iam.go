package resources

import (
	"fmt"
	"strings"

	"github.com/klothoplatform/klotho/pkg/core"
	"github.com/klothoplatform/klotho/pkg/sanitization/aws"
)

const (
	IAM_ROLE_TYPE      = "iam_role"
	IAM_POLICY_TYPE    = "iam_policy"
	OIDC_PROVIDER_TYPE = "iam_oidc_provider"
	VERSION            = "2012-10-17"
)

var roleSanitizer = aws.IamRoleSanitizer
var policySanitizer = aws.IamPolicySanitizer

var LAMBDA_ASSUMER_ROLE_POLICY = &PolicyDocument{
	Version: VERSION,
	Statement: []StatementEntry{
		{
			Action: []string{"sts:AssumeRole"},
			Principal: &Principal{
				Service: "lambda.amazonaws.com",
			},
			Effect: "Allow",
		},
	},
}

var ECS_ASSUMER_ROLE_POLICY = &PolicyDocument{
	Version: VERSION,
	Statement: []StatementEntry{
		{
			Action: []string{"sts:AssumeRole"},
			Principal: &Principal{
				Service: "ecs-tasks.amazonaws.com",
			},
			Effect: "Allow",
		},
	},
}

var EC2_ASSUMER_ROLE_POLICY = &PolicyDocument{
	Version: VERSION,
	Statement: []StatementEntry{
		{
			Action: []string{"sts:AssumeRole"},
			Principal: &Principal{
				Service: "ec2.amazonaws.com",
			},
			Effect: "Allow",
		},
	},
}

var EKS_FARGATE_ASSUME_ROLE_POLICY = &PolicyDocument{
	Version: VERSION,
	Statement: []StatementEntry{
		{
			Action: []string{"sts:AssumeRole"},
			Principal: &Principal{
				Service: "eks-fargate-pods.amazonaws.com",
			},
			Effect: "Allow",
		},
	},
}

var EKS_ASSUME_ROLE_POLICY = &PolicyDocument{
	Version: VERSION,
	Statement: []StatementEntry{
		{
			Action: []string{"sts:AssumeRole"},
			Principal: &Principal{
				Service: "eks.amazonaws.com",
			},
			Effect: "Allow",
		},
	},
}

type (
	IamRole struct {
		Name                string
		ConstructsRef       []core.AnnotationKey
		AssumeRolePolicyDoc *PolicyDocument `render:"document"`
		ManagedPolicies     []core.IaCValue
		AwsManagedPolicies  []string
		InlinePolicy        *PolicyDocument `render:"document"`
	}

	IamPolicy struct {
		Name          string
		ConstructsRef []core.AnnotationKey
		Policy        *PolicyDocument `render:"document"`
	}

	PolicyGenerator struct {
		unitToRole    map[string]*IamRole
		unitsPolicies map[string][]*IamPolicy
	}

	PolicyDocument struct {
		Version   string
		Statement []StatementEntry `render:"document"`
	}

	StatementEntry struct {
		Effect    string
		Action    []string
		Resource  []core.IaCValue
		Principal *Principal `render:"document"`
		Condition *Condition `render:"document"`
	}

	Principal struct {
		Service   string
		Federated core.IaCValue
	}

	Condition struct {
		StringEquals map[core.IaCValue]string
	}
)

func NewPolicyGenerator() *PolicyGenerator {
	p := &PolicyGenerator{
		unitsPolicies: make(map[string][]*IamPolicy),
		unitToRole:    make(map[string]*IamRole),
	}
	return p
}

func (p *PolicyGenerator) AddAllowPolicyToUnit(unitId string, policy *IamPolicy) {
	policies, found := p.unitsPolicies[unitId]
	if !found {
		p.unitsPolicies[unitId] = []*IamPolicy{policy}
	} else {
		for _, pol := range policies {
			if policy.Name == pol.Name {
				return
			}
		}
		p.unitsPolicies[unitId] = append(p.unitsPolicies[unitId], policy)
	}
}

func (p *PolicyGenerator) AddUnitRole(unitId string, role *IamRole) error {
	if _, found := p.unitToRole[unitId]; found {
		return fmt.Errorf("unit with id, %s, is already mapped to an IAM Role", unitId)
	}
	p.unitToRole[unitId] = role
	return nil
}

func (p *PolicyGenerator) GetUnitRole(unitId string) *IamRole {
	role := p.unitToRole[unitId]
	return role
}

func (p *PolicyGenerator) GetUnitPolicies(unitId string) []*IamPolicy {
	policies := p.unitsPolicies[unitId]
	return policies
}

func CreateAllowPolicyDocument(actions []string, resources []core.IaCValue) *PolicyDocument {
	return &PolicyDocument{
		Version: VERSION,
		Statement: []StatementEntry{
			{
				Effect:   "Allow",
				Action:   actions,
				Resource: resources,
			},
		},
	}
}

func NewIamRole(appName string, roleName string, ref []core.AnnotationKey, assumeRolePolicy *PolicyDocument) *IamRole {
	return &IamRole{
		Name:                roleSanitizer.Apply(fmt.Sprintf("%s-%s", appName, roleName)),
		ConstructsRef:       ref,
		AssumeRolePolicyDoc: assumeRolePolicy,
	}
}

// Provider returns name of the provider the resource is correlated to
func (role *IamRole) Provider() string {
	return AWS_PROVIDER
}

// KlothoResource returns AnnotationKey of the klotho resource the cloud resource is correlated to
func (role *IamRole) KlothoConstructRef() []core.AnnotationKey {
	return role.ConstructsRef
}

// ID returns the id of the cloud resource
func (role *IamRole) Id() string {
	return fmt.Sprintf("%s:%s:%s", role.Provider(), IAM_ROLE_TYPE, role.Name)
}

// ID returns the id of the cloud resource
func (role *IamRole) AddAwsManagedPolicies(policies []string) {
	role.AwsManagedPolicies = append(role.AwsManagedPolicies, policies...)
}

func NewIamPolicy(appName string, policyName string, ref core.AnnotationKey, policy *PolicyDocument) *IamPolicy {
	return &IamPolicy{
		Name:          policySanitizer.Apply(fmt.Sprintf("%s-%s", appName, policyName)),
		ConstructsRef: []core.AnnotationKey{ref},
		Policy:        policy,
	}
}

// Provider returns name of the provider the resource is correlated to
func (policy *IamPolicy) Provider() string {
	return AWS_PROVIDER
}

// KlothoResource returns AnnotationKey of the klotho resource the cloud resource is correlated to
func (policy *IamPolicy) KlothoConstructRef() []core.AnnotationKey {
	return policy.ConstructsRef
}

// ID returns the id of the cloud resource
func (policy *IamPolicy) Id() string {
	return fmt.Sprintf("%s:%s:%s", policy.Provider(), IAM_POLICY_TYPE, policy.Name)
}

func (s StatementEntry) Id() string {
	var id strings.Builder
	for _, r := range s.Resource {
		id.WriteString(r.Resource.Id())
		id.WriteRune(':')
		id.WriteString(r.Property)
	}
	id.WriteString("::")
	id.WriteString(s.Effect)
	id.WriteString("::")
	id.WriteString(strings.Join(s.Action, ","))

	return id.String()
}

func (d *PolicyDocument) Deduplicate() {
	keys := make(map[string]struct{})
	var unique []StatementEntry
	for _, stmt := range d.Statement {
		id := stmt.Id()
		if _, ok := keys[id]; !ok {
			keys[id] = struct{}{}
			unique = append(unique, stmt)
		}
	}
	d.Statement = unique
}
