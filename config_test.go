package ecschedule

import (
	"context"
	"os"
	"reflect"
	"testing"
	"text/template"

	"github.com/aws/aws-sdk-go/aws"
)

func TestLoadConfig(t *testing.T) {
	pathes := []string{"testdata/sample.yaml", "testdata/sample.json", "testdata/sample.jsonnet"}
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
					PropagateTags: aws.String("TASK_DEFINITION"),
					Role:          "ecsEventsRole",
				},
				BaseConfig: &BaseConfig{
					Region:    "us-east-1",
					Cluster:   "api",
					AccountID: "334",
				},
			},
		},
		Plugins:       []*Plugin(nil),
		templateFuncs: []template.FuncMap(nil),
		dir:           "testdata",
	}

	for _, path := range pathes {
		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		c, err := LoadConfig(context.Background(), f, "334", path)
		if err != nil {
			t.Errorf("error shoud be nil, but: %s", err)
		}

		if !reflect.DeepEqual(c, expect) {
			t.Errorf("unexpected output: %#v", c)
		}
	}
}

func TestLoadConfig_mustEnv(t *testing.T) {
	path := "testdata/sample2.yaml"
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	c, err := LoadConfig(context.Background(), f, "335", path)
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

func TestLoadConfig_tfstate(t *testing.T) {
	path := "testdata/sample3.yaml"
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	c, err := LoadConfig(context.Background(), f, "336", path)
	if err != nil {
		t.Errorf("error shoud be nil, but: %s", err)
	}

	if !reflect.DeepEqual(c.Plugins, []*Plugin{
		{Name: "tfstate", Config: map[string]interface{}{"path": "testdata/terraform.tfstate"}},
	}) {
		t.Errorf("unexpected output: %#v", c)
	}

	as := c.Rules[0].NetworkConfiguration.AwsVpcConfiguration.Subnets
	es := []string{"subnet-01234567", "subnet-12345678"}
	if !reflect.DeepEqual(as, es) {
		t.Errorf("error shoud be %v, but: %v", as, es)
	}

	asg := c.Rules[0].NetworkConfiguration.AwsVpcConfiguration.SecurityGroups
	esg := []string{"sg-11111111", "sg-99999999"}
	if !reflect.DeepEqual(asg, esg) {
		t.Errorf("error shoud be %v, but: %v", asg, esg)
	}
}

func TestLoadConfig_undefined(t *testing.T) {
	path := "testdata/sample4.yaml"
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	c, err := LoadConfig(context.Background(), f, "337", path)
	if err != nil {
		t.Errorf("error shoud be nil, but: %s", err)
	}

	if c.Rules[0].PropagateTags != nil {
		t.Errorf("error shoud be nil, but: %v", c.Rules[0].PropagateTags)
	}
}
