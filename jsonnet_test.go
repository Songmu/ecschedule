package ecschedule

import (
	"strings"
	"testing"
)

func TestJsonnetMustEnv(t *testing.T) {
	t.Setenv("ECSCHEDULE_TEST_ENV", "prod")
	vm := newJsonnetVM()
	out, err := vm.EvaluateAnonymousSnippet("test.jsonnet", `{
      v: std.native('must_env')('ECSCHEDULE_TEST_ENV'),
      isProd: std.native('must_env')('ECSCHEDULE_TEST_ENV') == 'prod',
    }`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"v": "prod"`) {
		t.Errorf("expected v=prod, got %s", out)
	}
	if !strings.Contains(out, `"isProd": true`) {
		t.Errorf("expected isProd=true, got %s", out)
	}
}

func TestJsonnetMustEnvUnset(t *testing.T) {
	vm := newJsonnetVM()
	_, err := vm.EvaluateAnonymousSnippet("test.jsonnet",
		`std.native('must_env')('ECSCHEDULE_TEST_UNSET_VAR_XYZ')`)
	if err == nil {
		t.Error("expected error for unset env var, got nil")
	}
}

func TestJsonnetEnv(t *testing.T) {
	t.Setenv("ECSCHEDULE_TEST_ENV", "production")
	vm := newJsonnetVM()
	out, err := vm.EvaluateAnonymousSnippet("test.jsonnet", `{
      set: std.native('env')('ECSCHEDULE_TEST_ENV', 'default'),
      unset: std.native('env')('ECSCHEDULE_TEST_UNSET_VAR_XYZ', 'default'),
    }`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"set": "production"`) {
		t.Errorf("expected set=production, got %s", out)
	}
	if !strings.Contains(out, `"unset": "default"`) {
		t.Errorf("expected unset=default, got %s", out)
	}
}

// Regression test for the boolean-field motivation: a non-string field
// (here, `disabled`) must be set at jsonnet evaluation time. The existing
// post-evaluation Go template substitution can only modify string values
// inside JSON, so it can't produce a JSON boolean for `Disabled bool`.
func TestJsonnetEnvBoolField(t *testing.T) {
	t.Setenv("ECSCHEDULE_TEST_ENV", "prod")
	vm := newJsonnetVM()
	out, err := vm.EvaluateAnonymousSnippet("test.jsonnet", `{
      disabled: std.native('must_env')('ECSCHEDULE_TEST_ENV') == 'prod',
    }`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"disabled": true`) {
		t.Errorf("expected disabled=true (boolean), got %s", out)
	}
}
