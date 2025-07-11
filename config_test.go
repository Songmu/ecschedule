package ecschedule

import (
	"context"
	"os"
	"reflect"
	"testing"
	"text/template"

	"github.com/aws/aws-sdk-go-v2/aws"
)

func TestLoadConfig(t *testing.T) {
	paths := []string{"testdata/sample.yaml", "testdata/sample.json", "testdata/sample.jsonnet"}
	expect := &Config{
		Role: "ecsEventsRole",
		BaseConfig: &BaseConfig{
			Region:     "us-east-1",
			Cluster:    "api",
			AccountID:  "334",
			TrackingID: "api",
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
					CapacityProviderStrategy: []*CapacityProviderStrategyItem{
						{
							CapacityProvider: "FARGATE",
							Weight:           1,
							Base:             1,
						},
					},
					NetworkConfiguration: &NetworkConfiguration{
						AwsVpcConfiguration: &AwsVpcConfiguration{
							Subnets:        []string{"subnet-01234567", "subnet-12345678"},
							SecurityGroups: []string{"sg-11111111", "sg-99999999"},
							AssignPublicIP: "ENABLED",
						},
					},
					TaskOverride: &TaskOverride{
						Cpu:    aws.String("4096"),
						Memory: aws.String("16384"),
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
					Region:     "us-east-1",
					Cluster:    "api",
					AccountID:  "334",
					TrackingID: "api",
				},
			},
		},
		Plugins:       []*Plugin(nil),
		templateFuncs: []template.FuncMap(nil),
		dir:           "testdata",
	}

	for _, path := range paths {
		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		c, err := LoadConfig(context.Background(), f, "334", path)
		if err != nil {
			t.Errorf("error should be nil, but: %s", err)
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
		t.Errorf("error should be nil, but: %s", err)
	}

	ru := c.GetRuleByName("hoge-task-name")
	err = ru.validateEnv()
	if err == nil {
		t.Errorf("error should be occurred but nil")
	}
	if g, e := err.Error(), "environment variable DUMMY_HOGE_ENV is not defined"; g != e {
		t.Errorf("error should be %q, but: %q", e, g)
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
		t.Errorf("error should be nil, but: %s", err)
	}

	if !reflect.DeepEqual(c.Plugins, []*Plugin{
		{Name: "tfstate", Config: map[string]interface{}{"path": "testdata/terraform.tfstate"}},
	}) {
		t.Errorf("unexpected output: %#v", c)
	}

	as := c.Rules[0].NetworkConfiguration.AwsVpcConfiguration.Subnets
	es := []string{"subnet-01234567", "subnet-12345678"}
	if !reflect.DeepEqual(as, es) {
		t.Errorf("error should be %v, but: %v", as, es)
	}

	asg := c.Rules[0].NetworkConfiguration.AwsVpcConfiguration.SecurityGroups
	esg := []string{"sg-11111111", "sg-99999999"}
	if !reflect.DeepEqual(asg, esg) {
		t.Errorf("error should be %v, but: %v", asg, esg)
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
		t.Errorf("error should be nil, but: %s", err)
	}

	if c.Rules[0].PropagateTags != nil {
		t.Errorf("error should be nil, but: %v", c.Rules[0].PropagateTags)
	}
}

func TestLoadConfig_tfstate_multi(t *testing.T) {
	path := "testdata/sample5.yaml"
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	c, err := LoadConfig(context.Background(), f, "338", path)
	if err != nil {
		t.Errorf("error should be nil, but: %s", err)
	}

	if !reflect.DeepEqual(c.Plugins, []*Plugin{
		{Name: "tfstate", Config: map[string]interface{}{"path": "testdata/terraform.tfstate"}, FuncPrefix: "first_"},
		{Name: "tfstate", Config: map[string]interface{}{"path": "testdata/terraform.tfstate"}, FuncPrefix: "second_"},
	}) {
		t.Errorf("unexpected output: %#v", c)
	}

	as := c.Rules[0].NetworkConfiguration.AwsVpcConfiguration.Subnets
	es := []string{"subnet-01234567", "subnet-12345678"}
	if !reflect.DeepEqual(as, es) {
		t.Errorf("error should be %v, but: %v", as, es)
	}

	asg := c.Rules[0].NetworkConfiguration.AwsVpcConfiguration.SecurityGroups
	esg := []string{"sg-11111111", "sg-99999999"}
	if !reflect.DeepEqual(asg, esg) {
		t.Errorf("error should be %v, but: %v", asg, esg)
	}
}

func TestCronValidate(t *testing.T) {
	c := &Config{
		Rules: []*Rule{
			{Name: "rule-1", ScheduleExpression: "cron(0 0 * * ? *)"},   // valid
			{Name: "rule-2", ScheduleExpression: "rate(1 day)"},         // rate expressions are excluded from validation
			{Name: "rule-3", ScheduleExpression: "invalid(0 0 * * *)"},  // invalid cron expression prefix
			{Name: "rule-4", ScheduleExpression: "cron(0 0 * * * *)"},   // missing '?'
			{Name: "rule-5", ScheduleExpression: "cron( 0 0 * * ? * )"}, // leading and trailing spaces are invalid but passes current cronplan.Parse()
		},
	}
	err := c.cronValidate()
	if err == nil {
		t.Errorf("error should be occurred, but nil")
	}

	// XXX: Handling or testing errors within the error message string is not a good approach,
	//      but we leave it as it is now.
	e := "schedule expression validation errors:\n" +
		"\trule \"rule-3\": invalid expression: \"invalid(0 0 * * *)\"\n" +
		"\trule \"rule-4\": either day-of-month or day-of-week must be '?'\n" +
		"\trule \"rule-5\": trailing or leading spaces are not allowed inside parentheses: \"cron( 0 0 * * ? * )\""
	if g := err.Error(); g != e {
		t.Errorf("unexpected error message\nwant:\n%s\n\ngot:\n%s", e, g)
	}
}
