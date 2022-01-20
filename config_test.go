package ecschedule

import (
	"os"
	"reflect"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	f, err := os.Open("testdata/sample.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	c, err := LoadConfig(f, "334")
	if err != nil {
		t.Errorf("error shoud be nil, but: %s", err)
	}

	expect := &Config{
		Role: "ecsEventsRole",
		BaseConfig: &BaseConfig{
			Region:    "us-east-1",
			Cluster:   "api",
			AccountID: "334",
		},
		Rules: []*Rule{
			{
				Name:               "hoge-task-name",
				Description:        "hoge description",
				ScheduleExpression: "cron(0 0 * * ? *)",
				Disabled:           false,
				Target: &Target{
					TargetID:        "",
					TaskDefinition:  "task1",
					TaskCount:       0,
					Group:           "xxx",
					PlatformVersion: "1.4.0",
					LaunchType:      "FARGATE",
					NetworkConfiguration: &NetworkConfiguration{
						AwsVpcConfiguration: &AwsVpcConfiguration{
							Subnets:        []string{"subnet-01234567", "subnet-12345678"},
							SecurityGroups: []string{"sg-11111111", "sg-99999999"},
							AssinPublicIP:  "ENABLED",
						},
					},
					ContainerOverrides: []*ContainerOverride{
						{
							Name: "container1",
							Command: []string{
								"subcmd",
								"argument",
							},
							Environment: map[string]string{
								"HOGE_ENV": "HOGEGE",
							},
						},
					},
					DeadLetterConfig: &DeadLetterConfig{
						Sqs: "queue1",
					},
					Role: "ecsEventsRole",
				},
				BaseConfig: &BaseConfig{
					Region:    "us-east-1",
					Cluster:   "api",
					AccountID: "334",
				},
			},
		},
	}

	if !reflect.DeepEqual(c, expect) {
		t.Errorf("unexpected output: %#v", c)
	}
}

func TestLoadConfig_mustEnv(t *testing.T) {
	f, err := os.Open("testdata/sample2.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	c, err := LoadConfig(f, "335")
	if err != nil {
		t.Errorf("error shoud be nil, but: %s", err)
	}

	ru := c.GetRuleByName("hoge-task-name")
	err = ru.validateEnv()
	if err == nil {
		t.Errorf("error should be occurred but nil")
	}
	if g, e := err.Error(), "environment variable DUMMY_HOGE_ENV is not defined"; g != e {
		t.Errorf("error shoud be %q, but: %q", e, g)
	}
}
