package ecschedule

import (
	"fmt"
	"os"

	"github.com/google/go-jsonnet"
	"github.com/google/go-jsonnet/ast"
)

// newJsonnetVM creates a jsonnet VM with native functions registered for
// accessing OS environment variables at jsonnet evaluation time.
//
// Registered native functions:
//   - must_env(name):     returns env var `name`; errors if unset.
//   - env(name, default): returns env var `name`, or `default` if unset.
//
// These mirror the Go template helpers in template.go. They are required
// for setting non-string fields (e.g., `disabled: bool`) conditionally per
// environment, which cannot be done via the post-evaluation Go template
// substitution because that operates on JSON byte streams (string values
// only).
func newJsonnetVM() *jsonnet.VM {
	vm := jsonnet.MakeVM()
	vm.NativeFunction(&jsonnet.NativeFunction{
		Name:   "must_env",
		Params: []ast.Identifier{"name"},
		Func: func(args []interface{}) (interface{}, error) {
			key, ok := args[0].(string)
			if !ok {
				return nil, fmt.Errorf("must_env: name must be a string")
			}
			v, ok := os.LookupEnv(key)
			if !ok {
				return nil, fmt.Errorf("must_env: environment variable %q is not set", key)
			}
			return v, nil
		},
	})
	vm.NativeFunction(&jsonnet.NativeFunction{
		Name:   "env",
		Params: []ast.Identifier{"name", "default"},
		Func: func(args []interface{}) (interface{}, error) {
			key, ok := args[0].(string)
			if !ok {
				return nil, fmt.Errorf("env: name must be a string")
			}
			if v, ok := os.LookupEnv(key); ok {
				return v, nil
			}
			return args[1], nil
		},
	})
	return vm
}
