package ecschedule

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"text/template"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/goccy/go-yaml"
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
		var opts []LoadConfigOption
		if filepath.Ext(path) == jsonnetExt {
			opts = append(opts,
				WithExtStr(map[string]string{
					"REGION":  "us-east-1",
					"CLUSTER": "api",
				}),
				WithExtCode(map[string]string{
					"BASE": "1",
				}),
			)
		}
		c, err := LoadConfig(context.Background(), f, "334", path, opts...)
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

func TestLoadConfig_tfstate_baseConfig(t *testing.T) {
	t.Chdir(t.TempDir())
	state := `{"version":4,"outputs":{"settings":{"value":{
		"region":"us-east-1","cluster":"api","role":"ecsEventsRole",
		"trackingId":"scheduled-tasks"
	},"type":["object",{"region":"string","cluster":"string","role":"string","trackingId":"string"}]}}}`
	statePath := "terraform.tfstate"
	if err := os.WriteFile(statePath, []byte(state), 0600); err != nil {
		t.Fatal(err)
	}

	for _, ext := range []string{".yaml", ".json", ".jsonnet"} {
		for _, explicitTrackingID := range []bool{false, true} {
			name := "default_tracking_id"
			if explicitTrackingID {
				name = "explicit_tracking_id"
			}
			t.Run(ext+"/"+name, func(t *testing.T) {
				conf := map[string]interface{}{
					"region":  "{{ tfstate `output.settings.region` }}",
					"cluster": "{{ tfstate `output.settings.cluster` }}",
					"role":    "{{ tfstate `output.settings.role` }}",
					"rules": []map[string]interface{}{
						{"name": "inherited", "scheduleExpression": "rate(1 day)", "taskDefinition": "task1"},
						{
							"name": "overridden", "scheduleExpression": "rate(1 day)", "taskDefinition": "task2",
							"region": "us-west-2", "cluster": "worker", "role": "workerRole", "trackingId": "worker-tasks",
						},
					},
					"plugins": []map[string]interface{}{
						{"name": "tfstate", "config": map[string]string{"path": statePath}},
					},
				}
				wantTrackingID := "api"
				if explicitTrackingID {
					conf["trackingId"] = "{{ tfstate `output.settings.trackingId` }}"
					wantTrackingID = "scheduled-tasks"
				}
				marshal := func(v interface{}) ([]byte, error) { return json.MarshalIndent(v, "", "  ") }
				if ext == ".yaml" {
					marshal = func(v interface{}) ([]byte, error) { return yaml.Marshal(v) }
				}
				data, err := marshal(conf)
				if err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(t.TempDir(), "ecschedule"+ext)
				if err := os.WriteFile(path, data, 0600); err != nil {
					t.Fatal(err)
				}
				c, err := LoadConfig(context.Background(), bytes.NewReader(data), "334", path)
				if err != nil {
					t.Fatal(err)
				}
				wantBase := &BaseConfig{
					Region: "us-east-1", Cluster: "api", AccountID: "334", TrackingID: wantTrackingID,
				}
				if !reflect.DeepEqual(c.BaseConfig, wantBase) {
					t.Errorf("base config = %#v, want %#v", c.BaseConfig, wantBase)
				}
				if c.Role != "ecsEventsRole" {
					t.Errorf("role = %q, want ecsEventsRole", c.Role)
				}
				for _, r := range c.Rules {
					if err := r.validateTFstate(); err != nil {
						t.Errorf("rule %s: %v", r.Name, err)
					}
					wantRole := "ecsEventsRole"
					want := wantBase
					if r.Name == "overridden" {
						wantRole = "workerRole"
						want = &BaseConfig{
							Region: "us-west-2", Cluster: "worker", AccountID: "334", TrackingID: "worker-tasks",
						}
					}
					if !reflect.DeepEqual(r.BaseConfig, want) || r.Role != wantRole {
						t.Errorf("rule %s: base = %#v, role = %q; want %#v, %q", r.Name, r.BaseConfig, r.Role, want, wantRole)
					}
				}
			})
		}
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

func TestLoadConfig_tfstate_multi_jsonnet(t *testing.T) {
	path := "testdata/sample5.jsonnet"
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	c, err := LoadConfig(context.Background(), f, "338", path)
	if err != nil {
		t.Fatalf("error should be nil, but: %s", err)
	}

	if !reflect.DeepEqual(c.Plugins, []*Plugin{
		{Name: "tfstate", Config: map[string]interface{}{"path": "testdata/terraform.tfstate"}, FuncPrefix: "first_"},
		{Name: "tfstate", Config: map[string]interface{}{"path": "testdata/terraform.tfstate"}, FuncPrefix: "second_"},
	}) {
		t.Errorf("unexpected output: %#v", c.Plugins)
	}
}

func TestLoadConfig_jsonnetExtVar_missing(t *testing.T) {
	// std.extVar references in sample.jsonnet must be supplied; otherwise
	// jsonnet should error rather than silently substituting an empty value.
	path := "testdata/sample.jsonnet"
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	if _, err := LoadConfig(context.Background(), f, "334", path); err == nil {
		t.Error("expected error when std.extVar bindings are missing, got nil")
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

func TestLoadConfigDuplicateRuleNames(t *testing.T) {
	conf := `region: us-east-1
cluster: api
rules:
- name: dup-task
  scheduleExpression: cron(0 0 * * ? *)
  taskDefinition: task1
- name: dup-task
  scheduleExpression: cron(5 0 * * ? *)
  taskDefinition: task2
`
	dir := t.TempDir()
	path := filepath.Join(dir, "dup.yaml")
	if err := os.WriteFile(path, []byte(conf), 0644); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	_, err = LoadConfig(context.Background(), f, "334", path)
	if err == nil {
		t.Fatal("expected duplicate rule name error, got nil")
	}
	if !strings.Contains(err.Error(), "dup-task") {
		t.Errorf("error should name the duplicate rule, got: %s", err)
	}
	if !strings.Contains(err.Error(), "ecschedule dump") {
		t.Errorf("error should mention the dump escape hatch, got: %s", err)
	}
}
