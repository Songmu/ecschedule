package ecsched

import (
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatchevents"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/ghodss/yaml"
)

type BaseConfig struct {
	Region    string `json:"region"`
	Cluster   string `json:"cluster"`
	Role      string `json:"role"`
	AccountID string `json:"-"`
}

type Config struct {
	*BaseConfig
	Rules []*Rule `json:"rules"`
}

type Rule struct {
	Name               string `json:"name"`
	Description        string `json:"description"`
	ScheduleExpression string `json:"scheduleExpression"`
	Disable            bool   `json:"disable"` // ENABLE | DISABLE
	*Target
	// Targets []Target

	*BaseConfig
}

type Target struct {
	TargetID           string               `json:"targetId,omitempty"`
	TaskDefinition     string               `json:"taskDefinition"`
	TaskCount          int64                `json:"taskCount,omitempty"`
	ContainerOverrides []*ContainerOverride `json:"containerOverrides"`
}

type ContainerOverride struct {
	Name        string            `json:"name"`
	Command     []string          `json:"command"` // ,flow
	Environment map[string]string `json:"environment"`
}

func (ta *Target) targetID(r *Rule) string {
	if r.TargetID == "" {
		return r.Name
	}
	return ta.TargetID
}

func (ta *Target) taskCount() int64 {
	if ta.TaskCount < 1 {
		return 1
	}
	return ta.TaskCount
}

func (r *Rule) roleARN() string {
	if strings.HasPrefix(r.Role, "arn:") {
		return r.Role
	}
	return fmt.Sprintf("arn:aws:iam::%s:role/%s", r.AccountID, r.Role)
}

func (r *Rule) ruleARN() string {
	return fmt.Sprintf("arn:aws:events:%s:%s:rule/%s", r.Region, r.AccountID, r.Name)
}

func (ta *Target) targetARN(r *Rule) string {
	if strings.HasPrefix(r.Cluster, "arn:") {
		return r.Cluster
	}
	return fmt.Sprintf("arn:aws:ecs:%s:%s:cluster/%s", r.Region, r.AccountID, r.Cluster)
}

func (ta *Target) taskDefinitionArn(r *Rule) string {
	if strings.HasPrefix(r.TaskDefinition, "arn:") {
		return r.TaskDefinition
	}
	return fmt.Sprintf("arn:aws:ecs:%s:%s:task-definition/%s", r.Region, r.AccountID, r.TaskDefinition)
}

func (r *Rule) state() string {
	if r.Disable {
		return "DISABLED"
	}
	return "ENABLED"
}

func (r *Rule) mergeBaseConfig(bc *BaseConfig) {
	if r.BaseConfig == nil {
		r.BaseConfig = bc
		return
	}
	if r.Region == "" {
		r.Region = bc.Region
	}
	if r.Role == "" {
		r.Role = bc.Role
	}
	if r.Cluster == "" {
		r.Cluster = bc.Cluster
	}
	if r.AccountID == "" {
		r.AccountID = bc.AccountID
	}
}

func (r *Rule) PutRuleInput() *cloudwatchevents.PutRuleInput {
	return &cloudwatchevents.PutRuleInput{
		Description:        aws.String(r.Description),
		Name:               aws.String(r.Name),
		RoleArn:            aws.String(r.roleARN()),
		ScheduleExpression: aws.String(r.ScheduleExpression),
		State:              aws.String(r.state()),
	}
}

func (r *Rule) PutTargetsInput() *cloudwatchevents.PutTargetsInput {
	return &cloudwatchevents.PutTargetsInput{
		Rule:    aws.String(r.Name),
		Targets: []*cloudwatchevents.Target{r.target()},
	}
}

func (r *Rule) target() *cloudwatchevents.Target {
	var containerOverrides []*ecs.ContainerOverride
	for _, co := range r.ContainerOverrides {
		var kvPairs []*ecs.KeyValuePair
		for k, v := range co.Environment {
			kvPairs = append(kvPairs, &ecs.KeyValuePair{
				Name:  aws.String(k),
				Value: aws.String(v),
			})
		}
		var cmds []*string
		for _, s := range co.Command {
			cmds = append(cmds, aws.String(s))
		}
		containerOverrides = append(containerOverrides, &ecs.ContainerOverride{
			Name:        aws.String(co.Name),
			Command:     cmds,
			Environment: kvPairs,
		})
	}

	return &cloudwatchevents.Target{
		Id:      aws.String(r.targetID(r)),
		Arn:     aws.String(r.targetARN(r)),
		RoleArn: aws.String(r.roleARN()),
		EcsParameters: &cloudwatchevents.EcsParameters{
			TaskDefinitionArn: aws.String(r.taskDefinitionArn(r)),
			TaskCount:         aws.Int64(r.taskCount()),
		},
		Input: aws.String("sss"),
	}
}

func LoadConfig(r io.Reader) (*Config, error) {
	c := Config{}
	bs, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(bs, &c); err != nil {
		return nil, err
	}
	return &c, nil
}
