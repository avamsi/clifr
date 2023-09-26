// Package climate "CLI Mate" provides a set of APIs to autogenerate CLIs from
// structs/functions with support for nested subcommands, global/local flags,
// help generation from comments, typo suggestions, shell completion and more.
//
// See https://github.com/avamsi/climate/blob/main/README.md for more details.
package climate

import (
	"context"
	"os"
	"os/exec"
	"reflect"

	"github.com/avamsi/ergo/assert"

	"github.com/avamsi/climate/internal"
)

type plan interface {
	execute(context.Context, *internal.Metadata) error
}

// Func returns an executable plan for the given function, which must conform to
// the following signatures (excuse the partial [optional] notation):
//
//	func([ctx context.Context], [opts *T], [args []string]) [(err error)]
//
// All of ctx, opts, args and error are optional. If opts is present, T must be
// a struct (whose fields are used as flags).
func Func(f any) *funcPlan {
	t := reflect.TypeOf(f)
	assert.Truef(t.Kind() == reflect.Func, "not a func: %v", t)
	v := reflect.ValueOf(f)
	return &funcPlan{reflection{ot: t, ov: &v}}
}

var _ plan = (*funcPlan)(nil)

// Struct returns an executable plan for the struct given as the type parameter,
// with its methods* (and "child" structs) as subcommands.
//
// * Only methods with pointer receiver are considered (and they must otherwise
// conform to the same signatures described in Func).
func Struct[T any](subcommands ...*structPlan) *structPlan {
	var (
		ptr = reflect.TypeOf((*T)(nil))
		t   = ptr.Elem()
	)
	assert.Truef(t.Kind() == reflect.Struct, "not a struct: %v", t)
	return &structPlan{
		reflection{ptr: &reflection{ot: ptr}, ot: t},
		subcommands,
	}
}

var _ plan = (*structPlan)(nil)

func exitCode(err error) int {
	if err == nil { // if _no_ error
		return 0
	}
	switch err := err.(type) {
	case *exitError:
		return err.code
	case *exec.ExitError:
		return err.ExitCode()
	default:
		return 1
	}
}

type runOptions struct {
	metadata *[]byte
}

// WithMetadata returns a modifier that sets the metadata to be used by Run for
// augmenting the CLI with additional information (for --help etc.).
func WithMetadata(b []byte) func(*runOptions) {
	return func(opts *runOptions) {
		opts.metadata = &b
	}
}

// Run executes the given plan and returns the exit code.
func Run(ctx context.Context, p plan, mods ...func(*runOptions)) int {
	var opts runOptions
	for _, mod := range mods {
		mod(&opts)
	}
	var md *internal.Metadata
	if opts.metadata != nil {
		md = internal.DecodeAsMetadata(*opts.metadata)
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	// Cobra already prints the error to stderr, so just return exit code here.
	return exitCode(p.execute(ctx, md))
}

// RunAndExit executes the given plan and exits with the exit code.
func RunAndExit(p plan, mods ...func(*runOptions)) {
	os.Exit(Run(context.Background(), p, mods...))
}
