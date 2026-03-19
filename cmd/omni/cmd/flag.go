// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/siderolabs/omni/internal/pkg/jsonschema"
)

// FlagBinder handles the deferred binding of Cobra flags to nil-able pointers.
// Flag names and descriptions are derived from the JSON schema field path.
type FlagBinder struct {
	cmd       *cobra.Command
	schema    *jsonschema.Schema
	callbacks []func() error
}

// NewFlagBinder creates a new binder for the given command.
func NewFlagBinder(cmd *cobra.Command, schema *jsonschema.Schema) *FlagBinder {
	return &FlagBinder{cmd: cmd, schema: schema}
}

// StringVar binds a string flag whose name and description are derived from the schema field path.
func (f *FlagBinder) StringVar(fieldPath string, target **string) {
	name, usage := f.mustFlagInfo(fieldPath)

	var val string

	f.cmd.Flags().StringVar(&val, name, "", usage)
	f.callbacks = append(f.callbacks, makeApplier(f.cmd, name, target, &val))
}

// BoolVar binds a bool flag whose name and description are derived from the schema field path.
func (f *FlagBinder) BoolVar(fieldPath string, ptr **bool) {
	name, usage := f.mustFlagInfo(fieldPath)

	var val bool

	f.cmd.Flags().BoolVar(&val, name, false, usage)
	f.callbacks = append(f.callbacks, makeApplier(f.cmd, name, ptr, &val))
}

// IntVar binds an int flag whose name and description are derived from the schema field path.
func (f *FlagBinder) IntVar(fieldPath string, target **int) {
	name, usage := f.mustFlagInfo(fieldPath)

	var val int

	f.cmd.Flags().IntVar(&val, name, 0, usage)
	f.callbacks = append(f.callbacks, makeApplier(f.cmd, name, target, &val))
}

// DurationVar binds a duration flag whose name and description are derived from the schema field path.
func (f *FlagBinder) DurationVar(fieldPath string, ptr **time.Duration) {
	name, usage := f.mustFlagInfo(fieldPath)

	var val time.Duration

	f.cmd.Flags().DurationVar(&val, name, 0, usage)
	f.callbacks = append(f.callbacks, makeApplier(f.cmd, name, ptr, &val))
}

// Uint32Var binds a uint32 flag whose name and description are derived from the schema field path.
func (f *FlagBinder) Uint32Var(fieldPath string, ptr **uint32) {
	name, usage := f.mustFlagInfo(fieldPath)

	var val uint32

	f.cmd.Flags().Uint32Var(&val, name, 0, usage)
	f.callbacks = append(f.callbacks, makeApplier(f.cmd, name, ptr, &val))
}

// Uint64Var binds a uint64 flag whose name and description are derived from the schema field path.
func (f *FlagBinder) Uint64Var(fieldPath string, ptr **uint64) {
	name, usage := f.mustFlagInfo(fieldPath)

	var val uint64

	f.cmd.Flags().Uint64Var(&val, name, 0, usage)
	f.callbacks = append(f.callbacks, makeApplier(f.cmd, name, ptr, &val))
}

// Float64Var binds a float64 flag whose name and description are derived from the schema field path.
func (f *FlagBinder) Float64Var(fieldPath string, ptr **float64) {
	name, usage := f.mustFlagInfo(fieldPath)

	var val float64

	f.cmd.Flags().Float64Var(&val, name, 0, usage)
	f.callbacks = append(f.callbacks, makeApplier(f.cmd, name, ptr, &val))
}

// EnumVar binds a string flag for an enum field, derived from the schema field path.
// Enum constraints are enforced later by JSON schema validation, not at flag parse time.
func EnumVar[T ~string](binder *FlagBinder, fieldPath string, target **T) {
	name, usage := binder.mustFlagInfo(fieldPath)

	var val string

	binder.cmd.Flags().StringVar(&val, name, "", usage)

	binder.callbacks = append(binder.callbacks, func() error {
		flag := binder.cmd.Flags().Lookup(name)
		if flag == nil {
			return fmt.Errorf("flag %q not found", name)
		}

		if flag.Changed {
			tVal := T(val)
			*target = &tVal
		}

		return nil
	})
}

// StringSliceVar binds a string slice flag whose name and description are derived from the schema field path.
func (f *FlagBinder) StringSliceVar(fieldPath string, target *[]string, defaultVal []string) {
	name, usage := f.mustFlagInfo(fieldPath)
	f.cmd.Flags().StringSliceVar(target, name, defaultVal, usage)
}

// ValueVar binds a custom pflag.Value flag whose name and description are derived from the schema field path.
func (f *FlagBinder) ValueVar(fieldPath string, value pflag.Value) {
	name, usage := f.mustFlagInfo(fieldPath)
	f.cmd.Flags().Var(value, name, usage)
}

// deprecatedStringAlias registers a deprecated string flag alias that points to the same target.
func (f *FlagBinder) deprecatedStringAlias(name, deprecationMsg string, target **string) {
	var val string

	f.cmd.Flags().StringVar(&val, name, "", "")

	//nolint:errcheck
	f.cmd.Flags().MarkDeprecated(name, deprecationMsg)

	f.callbacks = append(f.callbacks, makeApplier(f.cmd, name, target, &val))
}

// deprecatedIntAlias registers a deprecated int flag alias that points to the same target.
func (f *FlagBinder) deprecatedIntAlias(name, deprecationMsg string, target **int) {
	var val int

	f.cmd.Flags().IntVar(&val, name, 0, "")

	//nolint:errcheck
	f.cmd.Flags().MarkDeprecated(name, deprecationMsg)

	f.callbacks = append(f.callbacks, makeApplier(f.cmd, name, target, &val))
}

func (f *FlagBinder) BindAll() error {
	for _, cb := range f.callbacks {
		if err := cb(); err != nil {
			return fmt.Errorf("apply flags failed: %w", err)
		}
	}

	return nil
}

func (f *FlagBinder) mustFlagInfo(fieldPath string) (name, usage string) {
	return f.mustFlagName(fieldPath), f.mustFlagDescription(fieldPath)
}

func (f *FlagBinder) mustFlagName(fieldPath string) string {
	name := f.schema.FlagName("/" + strings.ReplaceAll(fieldPath, ".", "/"))
	if name == "" {
		panic("no x-cli-flag in schema for " + fieldPath)
	}

	return name
}

func (f *FlagBinder) mustFlagDescription(fieldPath string) string {
	description := f.schema.Description(fieldPath)
	if description == "" {
		panic("no description in schema for " + fieldPath)
	}

	// remove the first two words like "XYZ is" from the description
	parts := strings.SplitN(description, " ", 3)
	if len(parts) < 3 {
		return description
	}

	return strings.TrimSpace(parts[2])
}

// makeApplier creates the closure that assigns the temp flag value to the target pointer.
func makeApplier[T any](cmd *cobra.Command, name string, target **T, tempVal *T) func() error {
	return func() error {
		flag := cmd.Flags().Lookup(name)
		if flag == nil {
			return fmt.Errorf("flag %q not found", name)
		}

		if flag.Changed {
			*target = tempVal
		}

		return nil
	}
}
