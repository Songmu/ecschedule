package ecsched

import (
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatchevents"
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
	TaskCount          uint                 `json:"taskCount,omitempty"`
	ContainerOverrides []*ContainerOverride `json:"containerOverrides"`
}

type ContainerOverride struct {
	Name        string            `json:"name"`
	Command     []string          `json:"command"` // ,flow
	Environment map[string]string `json:"environment"`
}

func (r *Rule) targetID() string {
	if r.TargetID == "" {
		return r.Name
	}
	return r.TargetID
}

func (r *Rule) taskCount() uint {
	if r.TaskCount == 0 {
		return 1
	}
	return r.TaskCount
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

func (r *Rule) targetARN() string {
	if strings.HasPrefix(r.Cluster, "arn:") {
		return r.Cluster
	}
	return fmt.Sprintf("arn:aws:ecs:%s:%s:cluster/%s", r.Region, r.AccountID, r.Cluster)
}

func (r *Rule) taskDefinitionArn() string {
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
