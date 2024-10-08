import * as aws from '@pulumi/aws'
import * as awsInputs from '@pulumi/aws/types/input'
import { ModelCaseWrapper, TemplateWrapper } from '../../wrappers'

interface Args {
    Name: string
    Port: number
    Protocol: string
    Vpc: aws.ec2.Vpc
    TargetType: string
    Targets: { Id: string; Port: number }[]
    HealthCheck: TemplateWrapper<awsInputs.lb.TargetGroupHealthCheck>
    LambdaMultiValueHeadersEnabled?: boolean
    Tags: ModelCaseWrapper<Record<string, string>>
    Id: string
}

// noinspection JSUnusedLocalSymbols
function create(args: Args): aws.lb.TargetGroup {
    return (() => {
        const tg = new aws.lb.TargetGroup(args.Name, {
            port: args.Port,
            protocol: args.Protocol,
            targetType: args.TargetType,
            vpcId: args.Vpc.id,
            healthCheck: args.HealthCheck,
            //TMPL {{- if .LambdaMultiValueHeadersEnabled }}
            lambdaMultiValueHeadersEnabled: args.LambdaMultiValueHeadersEnabled,
            //TMPL {{- end }}
            //TMPL {{- if .Tags }}
            tags: args.Tags,
            //TMPL {{- end }}
        })

        //TMPL {{- if .Targets }}
        for (const target of args.Targets) {
            //TMPL {{- if eq .TargetType "instance" }}
            new aws.lb.TargetGroupAttachment(target.Id, {
                port: target.Port,
                targetGroupArn: tg.arn,
                targetId: target.Id,
            })
            //TMPL {{- else if eq .TargetType "lambda" }}
            new aws.lb.TargetGroupAttachment(target.Id, {
                targetGroupArn: tg.arn,
                targetId: target.Id,
            })
            //TMPL {{- end }}
        }
        //TMPL {{- end }}
        return tg
    })()
}

function properties(object: aws.lb.TargetGroup, args: Args) {
    return {
        Arn: object.arn,
    }
}

function importResource(args: Args): aws.lb.TargetGroup {
    return aws.lb.TargetGroup.get(args.Name, args.Id)
}
