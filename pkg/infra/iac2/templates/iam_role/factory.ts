import * as aws from '@pulumi/aws'
import * as pulumi from '@pulumi/pulumi'

interface Args {
    Name: string
    AssumeRolePolicyDoc: string
    InlinePolicy: aws.iam.PolicyDocument
    ManagedPolicies: pulumi.Output<string>[]
    AwsManagedPolicies: string[]
}

// noinspection JSUnusedLocalSymbols
function create(args: Args): aws.iam.Role {
    return new aws.iam.Role(args.Name, {
        assumeRolePolicy: JSON.parse(args.AssumeRolePolicyDoc),
        inlinePolicies: [
            {
                name: args.Name,
                policy: JSON.stringify(args.InlinePolicy),
            },
        ],
        managedPolicyArns: [...args.ManagedPolicies, ...args.AwsManagedPolicies],
    })
}