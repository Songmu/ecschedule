package ecschedule

import (
	"strings"
	"testing"
)

func TestRule_LocalYAMLForDiff_NormalizesEmptyRole(t *testing.T) {
	bc := &BaseConfig{
		Region:    "ap-northeast-1",
		Cluster:   "default",
		AccountID: "123456789012",
	}

	withEmptyRole := &Rule{
		Name:               "hello-world",
		ScheduleExpression: "cron(0 0 1 * ? *)",
		Target: &Target{
			TaskDefinition: "hello-world:1",
		},
		BaseConfig: bc,
	}
	withDefaultRole := &Rule{
		Name:               "hello-world",
		ScheduleExpression: "cron(0 0 1 * ? *)",
		Target: &Target{
			TaskDefinition: "hello-world:1",
			Role:           defaultRole,
		},
		BaseConfig: bc,
	}

	gotEmpty, err := withEmptyRole.localYAMLForDiff()
	if err != nil {
		t.Fatalf("localYAMLForDiff (empty role): %v", err)
	}
	gotDefault, err := withDefaultRole.localYAMLForDiff()
	if err != nil {
		t.Fatalf("localYAMLForDiff (default role): %v", err)
	}

	if gotEmpty != gotDefault {
		t.Errorf("YAML for empty role and default role should match.\nempty:\n%s\ndefault:\n%s", gotEmpty, gotDefault)
	}

	if !strings.Contains(gotEmpty, "role: "+defaultRole) {
		t.Errorf("expected normalized YAML to contain %q, got:\n%s", "role: "+defaultRole, gotEmpty)
	}
}

func TestRule_LocalYAMLForDiff_PreservesExplicitRole(t *testing.T) {
	bc := &BaseConfig{
		Region:    "ap-northeast-1",
		Cluster:   "default",
		AccountID: "123456789012",
	}

	const explicit = "service-role/AWS_Events_Invoke_Hello_World"
	r := &Rule{
		Name:               "hello-world",
		ScheduleExpression: "cron(0 0 1 * ? *)",
		Target: &Target{
			TaskDefinition: "hello-world:1",
			Role:           explicit,
		},
		BaseConfig: bc,
	}

	got, err := r.localYAMLForDiff()
	if err != nil {
		t.Fatalf("localYAMLForDiff: %v", err)
	}
	if !strings.Contains(got, "role: "+explicit) {
		t.Errorf("expected YAML to preserve explicit role %q, got:\n%s", explicit, got)
	}
}
