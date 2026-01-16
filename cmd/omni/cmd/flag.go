// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

// FlagBinder handles the deferred binding of Cobra flags to nil-able pointers.
type FlagBinder struct {
	cmd       *cobra.Command
	callbacks []func() error
}

// NewFlagBinder creates a new binder for the given command.
func NewFlagBinder(cmd *cobra.Command) *FlagBinder {
	return &FlagBinder{cmd: cmd}
}

func (f *FlagBinder) StringVar(name string, usage string, target **string) {
	var val string
	f.cmd.Flags().StringVar(&val, name, "", usage)
	f.callbacks = append(f.callbacks, makeApplier(f.cmd, name, target, &val))
}

func (f *FlagBinder) IntVar(name string, usage string, target **int) {
	var val int
	f.cmd.Flags().IntVar(&val, name, 0, usage)
	f.callbacks = append(f.callbacks, makeApplier(f.cmd, name, target, &val))
}

func (f *FlagBinder) BoolVar(name string, usage string, ptr **bool) {
	var val bool

	f.cmd.Flags().BoolVar(&val, name, false, usage)

	f.callbacks = append(f.callbacks, makeApplier(f.cmd, name, ptr, &val))
}

func (f *FlagBinder) DurationVar(name string, usage string, ptr **time.Duration) {
	var val time.Duration

	f.cmd.Flags().DurationVar(&val, name, 0, usage)

	f.callbacks = append(f.callbacks, makeApplier(f.cmd, name, ptr, &val))
}

func (f *FlagBinder) Uint64Var(name string, usage string, ptr **uint64) {
	var val uint64

	f.cmd.Flags().Uint64Var(&val, name, 0, usage)

	f.callbacks = append(f.callbacks, makeApplier(f.cmd, name, ptr, &val))
}

func (f *FlagBinder) Uint32Var(name string, usage string, ptr **uint32) {
	var val uint32

	f.cmd.Flags().Uint32Var(&val, name, 0, usage)

	f.callbacks = append(f.callbacks, makeApplier(f.cmd, name, ptr, &val))
}

func (f *FlagBinder) Float64Var(name string, usage string, ptr **float64) {
	var val float64

	f.cmd.Flags().Float64Var(&val, name, 0, usage)

	f.callbacks = append(f.callbacks, makeApplier(f.cmd, name, ptr, &val))
}

// EnumVar binds a string flag restricted to specific allowed values.
func EnumVar[T ~string](binder *FlagBinder, name string, usage string, target **T) {
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

func (f *FlagBinder) BindAll() error {
	for _, cb := range f.callbacks {
		if err := cb(); err != nil {
			return fmt.Errorf("apply flags failed: %w", err)
		}
	}

	return nil
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
