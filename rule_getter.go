package ecschedule

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatchevents"
	"github.com/aws/aws-sdk-go/service/ecs"
)

type ruleGetter struct {
	svc                                                                 *cloudwatchevents.CloudWatchEvents
	clusterArn, ruleArnPrefix, taskDefArnPrefix, roleArnPrefix, roleArn string
}

func (rg *ruleGetter) getRule(ctx context.Context, r *cloudwatchevents.Rule) (*Rule, error) {
	if !strings.HasPrefix(*r.Arn, rg.ruleArnPrefix) {
		return nil, nil
	}

	ta, err := rg.svc.ListTargetsByRuleWithContext(ctx, &cloudwatchevents.ListTargetsByRuleInput{
		Rule: r.Name,
	})
	if err != nil {
		return nil, err
	}
	var targets []*Target
	for _, t := range ta.Targets {
		if *t.Arn != rg.clusterArn {
			return nil, nil
		}
		targetID := *t.Id
		if targetID == *r.Name {
			targetID = ""
		}
		ecsParams := t.EcsParameters
		if ecsParams == nil {
			// ignore rule which have some non ecs targets
			return nil, nil
		}
		target := &Target{TargetID: targetID}

		if role := *t.RoleArn; role != rg.roleArn {
			target.Role = strings.TrimPrefix(role, rg.roleArnPrefix)
		}

		taskCount := *ecsParams.TaskCount
		if taskCount != 1 {
			target.TaskCount = taskCount
		}
		target.TaskDefinition = strings.TrimPrefix(*ecsParams.TaskDefinitionArn, rg.taskDefArnPrefix)

		target.Group = aws.StringValue(ecsParams.Group)
		target.LaunchType = aws.StringValue(ecsParams.LaunchType)
		target.PlatformVersion = aws.StringValue(ecsParams.PlatformVersion)
		if nc := ecsParams.NetworkConfiguration; nc != nil {
			target.NetworkConfiguration = &NetworkConfiguration{
				AwsVpcConfiguration: &AwsVpcConfiguration{
					Subnets:        aws.StringValueSlice(nc.AwsvpcConfiguration.Subnets),
					SecurityGroups: aws.StringValueSlice(nc.AwsvpcConfiguration.SecurityGroups),
					AssinPublicIP:  aws.StringValue(nc.AwsvpcConfiguration.AssignPublicIp),
				},
			}
		}

		taskOv := &ecs.TaskOverride{}
		if err := json.Unmarshal([]byte(*t.Input), taskOv); err != nil {
			return nil, err
		}
		var contOverrides []*ContainerOverride
		for _, co := range taskOv.ContainerOverrides {
			var cmd []string
			for _, c := range co.Command {
				cmd = append(cmd, *c)
			}
			env := map[string]string{}
			for _, kv := range co.Environment {
				env[*kv.Name] = *kv.Value
			}
			contOverrides = append(contOverrides, &ContainerOverride{
				Name:        *co.Name,
				Command:     cmd,
				Environment: env,
			})
		}
		target.ContainerOverrides = contOverrides
		targets = append(targets, target)
	}
	var desc string
	if r.Description != nil {
		desc = *r.Description
	}
	ru := &Rule{
		Name:               *r.Name,
		Description:        desc,
		ScheduleExpression: *r.ScheduleExpression,
		Disabled:           *r.State == "DISABLED",
	}
	switch len(targets) {
	case 0:
		return nil, nil
	case 1:
		ru.Target = targets[0]
	default:
		// not supported multiple target yet
		return nil, nil
		// ru.Target = targets[0]
		// ru.Targets = targets[1:]
	}
	return ru, nil
}
