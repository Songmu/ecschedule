package ecschedule

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchevents"
)

// stubHTTPClient satisfies aws.Config's HTTPClient interface and routes
// requests by the X-Amz-Target header (awsjson 1.1 protocol used by both
// CloudWatch Events and ECS). It records every request for assertions.
type stubHTTPClient struct {
	mu       sync.Mutex
	handlers map[string]func(body string) (status int, respBody string)
	requests []stubRequest
}

type stubRequest struct {
	target string
	body   string
}

func (s *stubHTTPClient) Do(req *http.Request) (*http.Response, error) {
	target := req.Header.Get("X-Amz-Target")
	var body string
	if req.Body != nil {
		bs, _ := io.ReadAll(req.Body)
		body = string(bs)
	}
	s.mu.Lock()
	s.requests = append(s.requests, stubRequest{target: target, body: body})
	h, ok := s.handlers[target]
	s.mu.Unlock()
	status, respBody := 400, `{"__type":"UnknownOperationException","message":"no stub handler"}`
	if ok {
		status, respBody = h(body)
	}
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/x-amz-json-1.1"}},
		Body:       io.NopCloser(strings.NewReader(respBody)),
	}, nil
}

func (s *stubHTTPClient) recorded() []stubRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]stubRequest(nil), s.requests...)
}

func stubAwsConfig(stub *stubHTTPClient) aws.Config {
	return aws.Config{
		Region: "us-east-1",
		Credentials: aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
			return aws.Credentials{AccessKeyID: "AKID", SecretAccessKey: "SECRET"}, nil
		}),
		HTTPClient:       stub,
		RetryMaxAttempts: 1,
	}
}

// stubTestRule returns a rule whose remote reconstruction from
// listRulesMatch + the ListTargetsByRule stub is byte-identical to its
// localYAMLForDiff, so diff yields from == to (the "no differences" case).
func stubTestRule() *Rule {
	return &Rule{
		Name:               "test-rule",
		ScheduleExpression: "cron(0 0 * * ? *)",
		Target: &Target{
			TaskDefinition: "task1",
			Role:           "ecsEventsRole",
		},
		BaseConfig: &BaseConfig{
			Region:     "us-east-1",
			Cluster:    "api",
			AccountID:  "334",
			TrackingID: "api",
		},
	}
}

const (
	listRulesMatch = `{"Rules":[{"Name":"test-rule","Arn":"arn:aws:events:us-east-1:334:rule/test-rule","ScheduleExpression":"cron(0 0 * * ? *)","State":"ENABLED"}]}`
	listRulesEmpty = `{"Rules":[]}`
)

func stubHandlers(listRulesResp string) map[string]func(string) (int, string) {
	return map[string]func(string) (int, string){
		"AmazonEC2ContainerServiceV20141113.DescribeTaskDefinition": func(string) (int, string) {
			return 200, `{"taskDefinition":{"taskDefinitionArn":"arn:aws:ecs:us-east-1:334:task-definition/task1:1"}}`
		},
		"AWSEvents.ListRules": func(string) (int, string) {
			return 200, listRulesResp
		},
		"AWSEvents.ListTargetsByRule": func(string) (int, string) {
			return 200, `{"Targets":[{"Id":"test-rule","Arn":"arn:aws:ecs:us-east-1:334:cluster/api","RoleArn":"arn:aws:iam::334:role/ecsEventsRole","EcsParameters":{"TaskCount":1,"TaskDefinitionArn":"arn:aws:ecs:us-east-1:334:task-definition/task1"}}]}`
		},
		"AWSEvents.PutRule": func(string) (int, string) {
			return 200, `{"RuleArn":"arn:aws:events:us-east-1:334:rule/test-rule"}`
		},
		"AWSEvents.PutTargets": func(string) (int, string) {
			return 200, `{"FailedEntryCount":0,"FailedEntries":[]}`
		},
		"AWSEvents.TagResource": func(string) (int, string) {
			return 200, `{}`
		},
	}
}

// captureLog redirects the global logger to a buffer with flags/prefix
// normalized the way cmd/ecschedule/main.go + Run set them at runtime.
// Tests using captureLog must not call t.Parallel(): the global logger is
// process-wide state.
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	origFlags := log.Flags()
	origPrefix := log.Prefix()
	log.SetFlags(0)
	log.SetPrefix("")
	log.SetOutput(&buf)
	t.Cleanup(func() {
		log.SetFlags(origFlags)
		log.SetPrefix(origPrefix)
		log.SetOutput(os.Stderr)
	})
	return &buf
}

func TestApplyGoldenLogNoDiff(t *testing.T) {
	stub := &stubHTTPClient{handlers: stubHandlers(listRulesMatch)}
	buf := captureLog(t)
	ru := stubTestRule()
	if err := ru.Apply(context.Background(), stubAwsConfig(stub), false); err != nil {
		t.Fatal(err)
	}
	want := "💡 skip applying. no differences\n"
	if got := buf.String(); got != want {
		t.Errorf("golden log mismatch\n got: %q\nwant: %q", got, want)
	}
	for _, req := range stub.recorded() {
		if strings.HasPrefix(req.target, "AWSEvents.Put") || req.target == "AWSEvents.TagResource" {
			t.Errorf("no-diff apply must not write, but called %s", req.target)
		}
	}
}

// canceledHTTPClient simulates the behavior of net/http.Client.Do when the
// request's context is already canceled at call time: it returns the
// context's error instead of a response. This lets us exercise the same
// smithy-go path (RequestSendError overridden with smithy.CanceledError)
// that a real SIGINT-during-DescribeTaskDefinition would take, without any
// real network I/O or timing dependency.
type canceledHTTPClient struct{}

func (canceledHTTPClient) Do(req *http.Request) (*http.Response, error) {
	if err := req.Context().Err(); err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("canceledHTTPClient: request context was not canceled")
}

// TestValidateTaskDefinitionPreservesContextCanceled proves the %w wrap in
// validateTaskDefinition keeps the error chain intact, so
// errors.Is(err, context.Canceled) still holds after wrapping. This is what
// parallel_apply.go's Phase-1 classification relies on to report a SIGINT
// during planning as "interrupted", not as a validation failure.
func TestValidateTaskDefinitionPreservesContextCanceled(t *testing.T) {
	r := stubTestRule()
	awsConf := stubAwsConfig(&stubHTTPClient{})
	awsConf.HTTPClient = canceledHTTPClient{}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := r.validateTaskDefinition(ctx, awsConf)
	if err == nil {
		t.Fatal("expected an error when the context is already canceled")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("errors.Is(err, context.Canceled) = false, want true (error chain broken); err: %v", err)
	}
}

// logLine mimics log.Output: appends a newline only when missing.
func logLine(s string) string {
	if strings.HasSuffix(s, "\n") {
		return s
	}
	return s + "\n"
}

func TestApplyGoldenLogDiffDryRun(t *testing.T) {
	stub := &stubHTTPClient{handlers: stubHandlers(listRulesEmpty)}
	buf := captureLog(t)
	ru := stubTestRule()
	if err := ru.Apply(context.Background(), stubAwsConfig(stub), true); err != nil {
		t.Fatal(err)
	}
	localYaml, err := ru.localYAMLForDiff()
	if err != nil {
		t.Fatal(err)
	}
	diffOut := formatDiff("test-rule", "", localYaml, diffFormatPrettyColored)
	want := logLine(fmt.Sprintf("💡 applying following changes (dry-run)\n%s", diffOut))
	if got := buf.String(); got != want {
		t.Errorf("golden log mismatch\n got: %q\nwant: %q", got, want)
	}
	for _, req := range stub.recorded() {
		if strings.HasPrefix(req.target, "AWSEvents.Put") || req.target == "AWSEvents.TagResource" {
			t.Errorf("dry-run must not write, but called %s", req.target)
		}
	}
}

func TestApplyGoldenLogDiffRealApply(t *testing.T) {
	stub := &stubHTTPClient{handlers: stubHandlers(listRulesEmpty)}
	buf := captureLog(t)
	ru := stubTestRule()
	if err := ru.Apply(context.Background(), stubAwsConfig(stub), false); err != nil {
		t.Fatal(err)
	}
	localYaml, err := ru.localYAMLForDiff()
	if err != nil {
		t.Fatal(err)
	}
	diffOut := formatDiff("test-rule", "", localYaml, diffFormatPrettyColored)
	want := logLine(fmt.Sprintf("💡 applying following changes\n%s", diffOut))
	if got := buf.String(); got != want {
		t.Errorf("golden log mismatch\n got: %q\nwant: %q", got, want)
	}
	var wrote []string
	for _, req := range stub.recorded() {
		if strings.HasPrefix(req.target, "AWSEvents.Put") || req.target == "AWSEvents.TagResource" {
			wrote = append(wrote, req.target)
		}
	}
	wantCalls := []string{"AWSEvents.PutRule", "AWSEvents.PutTargets", "AWSEvents.TagResource"}
	if strings.Join(wrote, ",") != strings.Join(wantCalls, ",") {
		t.Errorf("write calls = %v, want %v", wrote, wantCalls)
	}
}

func TestApplyPutTargetsKeepsEnvironment(t *testing.T) {
	stub := &stubHTTPClient{handlers: stubHandlers(listRulesEmpty)}
	captureLog(t)
	ru := stubTestRule()
	ru.ContainerOverrides = []*ContainerOverride{
		{Name: "container1", Environment: map[string]string{"FOO": "bar-env-value"}},
	}
	if err := ru.Apply(context.Background(), stubAwsConfig(stub), false); err != nil {
		t.Fatal(err)
	}
	var putTargetsBody string
	for _, req := range stub.recorded() {
		if req.target == "AWSEvents.PutTargets" {
			putTargetsBody = req.body
		}
	}
	if putTargetsBody == "" {
		t.Fatal("PutTargets was not called")
	}
	if !strings.Contains(putTargetsBody, "bar-env-value") {
		t.Errorf("PutTargets input lost the container-override environment: %s", putTargetsBody)
	}
}

func TestExecuteRetriesConcurrentModification(t *testing.T) {
	var calls int
	handlers := stubHandlers(listRulesEmpty)
	handlers["AWSEvents.TagResource"] = func(string) (int, string) {
		calls++
		if calls < 3 {
			return 400, `{"__type":"ConcurrentModificationException","message":"concurrent modification"}`
		}
		return 200, `{}`
	}
	stub := &stubHTTPClient{handlers: handlers}
	captureLog(t)
	ru := stubTestRule()
	svc := cloudwatchevents.NewFromConfig(stubAwsConfig(stub), func(o *cloudwatchevents.Options) {
		o.Region = ru.Region
	})
	if err := ru.execute(context.Background(), svc); err != nil {
		t.Fatalf("execute should succeed after retries, got: %s", err)
	}
	if calls != 3 {
		t.Errorf("TagResource calls = %d, want 3", calls)
	}
}

func TestExecuteTagFailureReturnsDedicatedError(t *testing.T) {
	var calls int
	handlers := stubHandlers(listRulesEmpty)
	handlers["AWSEvents.TagResource"] = func(string) (int, string) {
		calls++
		return 400, `{"__type":"AccessDeniedException","message":"denied"}`
	}
	stub := &stubHTTPClient{handlers: handlers}
	captureLog(t)
	ru := stubTestRule()
	svc := cloudwatchevents.NewFromConfig(stubAwsConfig(stub), func(o *cloudwatchevents.Options) {
		o.Region = ru.Region
	})
	err := ru.execute(context.Background(), svc)
	if err == nil {
		t.Fatal("expected error")
	}
	if calls != 1 {
		t.Errorf("non-retryable tag error should not be retried at app level: calls = %d, want 1", calls)
	}
	var tagErr *tagResourceError
	if !errors.As(err, &tagErr) {
		t.Fatalf("error should be *tagResourceError, got %T: %s", err, err)
	}
	if tagErr.Error() != tagErr.err.Error() {
		t.Errorf("tagResourceError.Error() must equal the wrapped error string\n got: %q\nwant: %q", tagErr.Error(), tagErr.err.Error())
	}
	if tagErr.ruleARN != "arn:aws:events:us-east-1:334:rule/test-rule" {
		t.Errorf("ruleARN = %q", tagErr.ruleARN)
	}
	if tagErr.trackingID != "api" {
		t.Errorf("trackingID = %q", tagErr.trackingID)
	}
}
