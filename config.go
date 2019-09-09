package ecsched

import (
	"fmt"
	"strings"
)

type BaseConfig struct {
	Region    string `yaml:"regison"`
	Cluster   string `yaml:"cluster"`
	Role      string `yaml:"role"`
	AccountID string `yaml:"-"`
}

type Config struct {
	*BaseConfig
	Rules []*Rule `yaml:"rules"`
}

type Rule struct {
	Name               string `yaml:"name"`
	Description        string `yaml:"description"`
	ScheduleExpression string `yaml:"scheduleExpression"`
	Disable            bool   `yaml:"disable"` // ENABLE | DISABLE
	*Target
	// Targets []Target

	*BaseConfig
}

type Target struct {
	TargetID           string               `yaml:"targetId,omitempty"`
	TaskDefinition     string               `yaml:"taskDefinition"`
	TaskCount          uint                 `yaml:"taskCount,omitempty"`
	ContainerOverrides []*ContainerOverride `yaml:"containerOverrides"`
}

type ContainerOverride struct {
	Name        string            `yaml:"name"`
	Command     []string          `yaml:"command"`
	Environment map[string]string `yaml:"environment"`
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
