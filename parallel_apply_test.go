package ecschedule

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
)

func parallelTestConfig(rules ...*Rule) *Config {
	return &Config{
		BaseConfig: &BaseConfig{
			Region:     "us-east-1",
			Cluster:    "api",
			AccountID:  "334",
			TrackingID: "api",
		},
		Rules: rules,
	}
}

func parallelTestRule(name string) *Rule {
	ru := stubTestRule()
	ru.Name = name
	return ru
}

func TestFormatApplySummary(t *testing.T) {
	failed := []jobOutcome[string]{
		{Index: 0, Name: "job-a", Err: errors.New("PutTargets: AccessDenied")},
	}
	got := formatApplySummary(8, 2, failed, []string{"job-f", "job-g"})
	want := "apply summary: 8 applied, 2 no change, 1 failed, 2 not started\n" +
		"failed:\n" +
		"  - job-a: PutTargets: AccessDenied\n" +
		"not started (interrupted): job-f, job-g"
	if got != want {
		t.Errorf("summary mismatch\n got: %q\nwant: %q", got, want)
	}
	gotOK := formatApplySummary(3, 1, nil, nil)
	wantOK := "apply summary: 3 applied, 1 no change, 0 failed, 0 not started"
	if gotOK != wantOK {
		t.Errorf("summary mismatch\n got: %q\nwant: %q", gotOK, wantOK)
	}
}

func TestRunParallelApplyAppliesAllRules(t *testing.T) {
	stub := &stubHTTPClient{handlers: stubHandlers(listRulesEmpty)}
	buf := captureLog(t)
	r1, r2 := parallelTestRule("rule-one"), parallelTestRule("rule-two")
	r1.ContainerOverrides = []*ContainerOverride{
		{Name: "container1", Environment: map[string]string{"FOO": "bar-env-value"}},
	}
	c := parallelTestConfig(r1, r2)
	err := runParallelApply(context.Background(), stubAwsConfig(stub),
		c, []string{"rule-one", "rule-two"}, 2, diffFormatPrettyColored)
	if err != nil {
		t.Fatalf("parallel apply should succeed, got: %s", err)
	}
	// PutTargets must carry the environment: display mutation happens
	// only after execute returns for that rule.
	var putTargetsBodies []string
	putRules := 0
	for _, req := range stub.recorded() {
		switch req.target {
		case "AWSEvents.PutTargets":
			putTargetsBodies = append(putTargetsBodies, req.body)
		case "AWSEvents.PutRule":
			putRules++
		}
	}
	if putRules != 2 {
		t.Errorf("PutRule calls = %d, want 2", putRules)
	}
	found := false
	for _, body := range putTargetsBodies {
		if strings.Contains(body, "bar-env-value") {
			found = true
		}
	}
	if !found {
		t.Error("PutTargets input lost the container-override environment")
	}
	out := buf.String()
	if !strings.Contains(out, "apply summary: 2 applied, 0 no change, 0 failed, 0 not started") {
		t.Errorf("missing summary in output:\n%s", out)
	}
}

func TestRunParallelApplyNoChangeCountsAsSuccess(t *testing.T) {
	stub := &stubHTTPClient{handlers: stubHandlers(listRulesMatch)}
	buf := captureLog(t)
	// stubTestRule matches the remote reconstruction → no diff
	ru := parallelTestRule("test-rule")
	c := parallelTestConfig(ru)
	err := runParallelApply(context.Background(), stubAwsConfig(stub),
		c, []string{"test-rule"}, 2, diffFormatPrettyColored)
	if err != nil {
		t.Fatalf("no-change run must return nil (prune gate), got: %s", err)
	}
	for _, req := range stub.recorded() {
		if strings.HasPrefix(req.target, "AWSEvents.Put") || req.target == "AWSEvents.TagResource" {
			t.Errorf("no-change rule must not be written: %s", req.target)
		}
	}
	if !strings.Contains(buf.String(), "apply summary: 0 applied, 1 no change, 0 failed, 0 not started") {
		t.Errorf("summary mismatch:\n%s", buf.String())
	}
}

func TestRunParallelApplyValidationErrorBlocksAllWrites(t *testing.T) {
	handlers := stubHandlers(listRulesEmpty)
	handlers["AmazonEC2ContainerServiceV20141113.DescribeTaskDefinition"] = func(body string) (int, string) {
		if strings.Contains(body, "task-broken") {
			return 400, `{"__type":"ClientException","message":"Unable to describe task definition."}`
		}
		return 200, `{"taskDefinition":{"taskDefinitionArn":"arn:aws:ecs:us-east-1:334:task-definition/task1:1"}}`
	}
	stub := &stubHTTPClient{handlers: handlers}
	buf := captureLog(t)
	good, bad := parallelTestRule("rule-good"), parallelTestRule("rule-bad")
	bad.TaskDefinition = "task-broken"
	c := parallelTestConfig(good, bad)
	err := runParallelApply(context.Background(), stubAwsConfig(stub),
		c, []string{"rule-good", "rule-bad"}, 2, diffFormatPrettyColored)
	if err == nil {
		t.Fatal("expected planning failure")
	}
	if !strings.Contains(err.Error(), "1 rule(s) failed validation/planning") {
		t.Errorf("err = %s", err)
	}
	for _, req := range stub.recorded() {
		if strings.HasPrefix(req.target, "AWSEvents.Put") || req.target == "AWSEvents.TagResource" {
			t.Errorf("planning failure must block every write, but saw %s", req.target)
		}
	}
	if !strings.Contains(buf.String(), `rule "rule-bad"`) {
		t.Errorf("plan error must name the failing rule:\n%s", buf.String())
	}
}

func TestRunParallelApplyPreCanceledCtxIsInterrupted(t *testing.T) {
	stub := &stubHTTPClient{handlers: stubHandlers(listRulesEmpty)}
	captureLog(t)
	ru := parallelTestRule("rule-one")
	c := parallelTestConfig(ru)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := runParallelApply(ctx, stubAwsConfig(stub),
		c, []string{"rule-one"}, 2, diffFormatPrettyColored)
	if err == nil {
		t.Fatal("interrupted run must return an error (prune gate)")
	}
	if !strings.Contains(err.Error(), "interrupted") {
		t.Errorf("err = %s", err)
	}
	if reqs := stub.recorded(); len(reqs) != 0 {
		t.Errorf("no AWS call should happen when canceled before planning: %v", reqs)
	}
}

func TestRunParallelApplyExecFailureAggregates(t *testing.T) {
	handlers := stubHandlers(listRulesEmpty)
	handlers["AWSEvents.PutTargets"] = func(body string) (int, string) {
		if strings.Contains(body, "rule-bad") {
			return 400, `{"__type":"AccessDeniedException","message":"denied"}`
		}
		return 200, `{"FailedEntryCount":0,"FailedEntries":[]}`
	}
	stub := &stubHTTPClient{handlers: handlers}
	buf := captureLog(t)
	good, bad := parallelTestRule("rule-good"), parallelTestRule("rule-bad")
	c := parallelTestConfig(good, bad)
	err := runParallelApply(context.Background(), stubAwsConfig(stub),
		c, []string{"rule-good", "rule-bad"}, 2, diffFormatPrettyColored)
	if err == nil {
		t.Fatal("expected failure")
	}
	if err.Error() != "1 rule(s) failed" {
		t.Errorf("sentinel error = %q, want %q", err.Error(), "1 rule(s) failed")
	}
	out := buf.String()
	if !strings.Contains(out, "apply summary: 1 applied, 0 no change, 1 failed, 0 not started") {
		t.Errorf("summary mismatch:\n%s", out)
	}
	if !strings.Contains(out, "✅ rule \"rule-good\" applied") {
		t.Errorf("successful rule block missing:\n%s", out)
	}
}

// TestRunParallelApplyValidatesEachTaskDefinitionOnce pins the memoization
// of task-definition validation. Real configs point many rules at a handful
// of task definitions; without this, planning issues one identical
// DescribeTaskDefinition per rule and ECS throttles the run before any
// write happens (observed against a 150-rule config at -parallel 10).
func TestRunParallelApplyValidatesEachTaskDefinitionOnce(t *testing.T) {
	stub := &stubHTTPClient{handlers: stubHandlers(listRulesEmpty)}
	captureLog(t)
	var rules []*Rule
	var names []string
	for i := 0; i < 20; i++ {
		name := fmt.Sprintf("rule-%02d", i)
		ru := parallelTestRule(name)
		if i >= 10 {
			ru.TaskDefinition = "task2"
		}
		rules = append(rules, ru)
		names = append(names, name)
	}
	c := parallelTestConfig(rules...)
	if err := runParallelApply(context.Background(), stubAwsConfig(stub),
		c, names, 10, diffFormatPrettyColored); err != nil {
		t.Fatalf("parallel apply should succeed, got: %s", err)
	}
	describes := 0
	for _, req := range stub.recorded() {
		if req.target == "AmazonEC2ContainerServiceV20141113.DescribeTaskDefinition" {
			describes++
		}
	}
	if describes != 2 {
		t.Errorf("DescribeTaskDefinition calls = %d, want 2: 20 rules share 2 task definitions", describes)
	}
}

// TestApplyDryRunParallelValidatesEachTaskDefinitionOnce covers the same
// amplification on the dry-run path, which does not go through
// runParallelApply: it fans applyInternal out over executeJobsInParallel,
// so the memoization has to be shared by the caller rather than created
// per rule.
func TestApplyDryRunParallelValidatesEachTaskDefinitionOnce(t *testing.T) {
	stub := &stubHTTPClient{handlers: stubHandlers(listRulesEmpty)}
	captureLog(t)
	var rules []*Rule
	for i := 0; i < 20; i++ {
		rules = append(rules, parallelTestRule(fmt.Sprintf("rule-%02d", i)))
	}
	ctx := setApp(context.Background(), &app{
		AccountID: "334",
		AwsConf:   stubAwsConfig(stub),
		Config:    parallelTestConfig(rules...),
	})
	err := cmdApply.Run(ctx, []string{"-all", "-dry-run", "-parallel", "10"}, io.Discard, io.Discard)
	if err != nil {
		t.Fatalf("dry-run should succeed, got: %s", err)
	}
	describes := 0
	for _, req := range stub.recorded() {
		if req.target == "AmazonEC2ContainerServiceV20141113.DescribeTaskDefinition" {
			describes++
		}
	}
	if describes != 1 {
		t.Errorf("DescribeTaskDefinition calls = %d, want 1: 20 rules share one task definition", describes)
	}
}

func TestApplyCmdParallelGuards(t *testing.T) {
	captureLog(t)
	ctx := setApp(context.Background(), &app{AccountID: "334"})
	// parallel < 1 is still rejected
	err := cmdApply.Run(ctx, []string{"-rule", "x", "-parallel", "0"}, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "-parallel must be at least 1") {
		t.Errorf("parallel=0 should be rejected, got: %v", err)
	}
	// parallel > 1 without -dry-run is no longer rejected up front
	// (it fails later for a different reason: no config loaded).
	err = cmdApply.Run(ctx, []string{"-rule", "x", "-parallel", "2"}, io.Discard, io.Discard)
	if err == nil {
		t.Fatal("expected an error (no config), got nil")
	}
	if strings.Contains(err.Error(), "can only be used with -dry-run") {
		t.Errorf("the dry-run-only guard must be removed, got: %v", err)
	}
}
