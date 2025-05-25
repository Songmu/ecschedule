package ecschedule

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchevents"
	cweTypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchevents/types"
	ecsTypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

type ruleGetter struct {
	svc                                                                               *cloudwatchevents.Client
	clusterArn, ruleArnPrefix, taskDefArnPrefix, roleArnPrefix, roleArn, sqsArnPrefix string
}

func (rg *ruleGetter) getRule(ctx context.Context, r *cweTypes.Rule) (*Rule, error) {
	if !strings.HasPrefix(*r.Arn, rg.ruleArnPrefix) {
		return nil, nil
	}

	ta, err := rg.svc.ListTargetsByRule(ctx, &cloudwatchevents.ListTargetsByRuleInput{
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

		target.Group = aws.ToString(ecsParams.Group)
		target.LaunchType = string(ecsParams.LaunchType)

		var capacityProviderStrategy []*CapacityProviderStrategyItem
		for _, cps := range ecsParams.CapacityProviderStrategy {
			capacityProviderStrategy = append(capacityProviderStrategy, &CapacityProviderStrategyItem{
				Base:             cps.Base,
				Weight:           cps.Weight,
				CapacityProvider: aws.ToString(cps.CapacityProvider),
			})
		}

		target.CapacityProviderStrategy = capacityProviderStrategy

		target.PlatformVersion = aws.ToString(ecsParams.PlatformVersion)
		target.PropagateTags = aws.String(string(t.EcsParameters.PropagateTags))
		if aws.ToString(target.PropagateTags) == "" {
			target.PropagateTags = nil
		}
		if nc := ecsParams.NetworkConfiguration; nc != nil {
			target.NetworkConfiguration = &NetworkConfiguration{
				AwsVpcConfiguration: &AwsVpcConfiguration{
					Subnets:        nc.AwsvpcConfiguration.Subnets,
					SecurityGroups: nc.AwsvpcConfiguration.SecurityGroups,
					AssignPublicIP: string(nc.AwsvpcConfiguration.AssignPublicIp),
				},
			}
		}

		// For backward-compatibility, ContainerOverrides and TaskOverride are held as separate fields.
		taskOv := &ecsTypes.TaskOverride{}
		if t.Input != nil {
			if err := json.Unmarshal([]byte(*t.Input), taskOv); err != nil {
				return nil, err
			}
			if taskOv.Cpu != nil {
				target.TaskOverride.Cpu = *taskOv.Cpu
			}
			if taskOv.Memory != nil {
				target.TaskOverride.Memory = *taskOv.Memory
			}
			var contOverrides []*ContainerOverride
			for _, co := range taskOv.ContainerOverrides {
				var cmd []string
				for _, c := range co.Command {
					cmd = append(cmd, c)
				}
				env := map[string]string{}
				for _, kv := range co.Environment {
					env[*kv.Name] = *kv.Value
				}
				contOverrides = append(contOverrides, &ContainerOverride{
					Name:              *co.Name,
					Command:           cmd,
					Environment:       env,
					Cpu:               co.Cpu,
					Memory:            co.Memory,
					MemoryReservation: co.MemoryReservation,
				})
			}
			target.ContainerOverrides = contOverrides
		}
		targets = append(targets, target)

		if dlc := t.DeadLetterConfig; dlc != nil {
			target.DeadLetterConfig = &DeadLetterConfig{
				Sqs: strings.TrimPrefix(*dlc.Arn, rg.sqsArnPrefix),
			}
		}
	}
	var desc string
	if r.Description != nil {
		desc = *r.Description
	}
	ru := &Rule{
		Name:               *r.Name,
		Description:        desc,
		ScheduleExpression: *r.ScheduleExpression,
		Disabled:           string(r.State) == "DISABLED",
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
