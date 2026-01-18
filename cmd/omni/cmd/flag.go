// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

// flagBinder handles the deferred binding of Cobra flags to nil-able pointers.
type flagBinder struct {
	cmd       *cobra.Command
	callbacks []func() error
}

// newFlagBinder creates a new binder for the given command.
func newFlagBinder(cmd *cobra.Command) *flagBinder {
	return &flagBinder{cmd: cmd}
}

func (f *flagBinder) stringVar(name string, usage string, target **string) {
	var val string
	f.cmd.Flags().StringVar(&val, name, "", usage)
	f.callbacks = append(f.callbacks, makeApplier(f.cmd, name, target, &val))
}

func (f *flagBinder) intVar(name string, usage string, target **int) {
	var val int
	f.cmd.Flags().IntVar(&val, name, 0, usage)
	f.callbacks = append(f.callbacks, makeApplier(f.cmd, name, target, &val))
}

func (f *flagBinder) boolVar(name string, usage string, ptr **bool) {
	var val bool

	f.cmd.Flags().BoolVar(&val, name, false, usage)

	f.callbacks = append(f.callbacks, makeApplier(f.cmd, name, ptr, &val))
}

func (f *flagBinder) durationVar(name string, usage string, ptr **time.Duration) {
	var val time.Duration

	f.cmd.Flags().DurationVar(&val, name, 0, usage)

	f.callbacks = append(f.callbacks, makeApplier(f.cmd, name, ptr, &val))
}

func (f *flagBinder) uint64Var(name string, usage string, ptr **uint64) {
	var val uint64

	f.cmd.Flags().Uint64Var(&val, name, 0, usage)

	f.callbacks = append(f.callbacks, makeApplier(f.cmd, name, ptr, &val))
}

func (f *flagBinder) uint32Var(name string, usage string, ptr **uint32) {
	var val uint32

	f.cmd.Flags().Uint32Var(&val, name, 0, usage)

	f.callbacks = append(f.callbacks, makeApplier(f.cmd, name, ptr, &val))
}

func (f *flagBinder) float64Var(name string, usage string, ptr **float64) {
	var val float64

	f.cmd.Flags().Float64Var(&val, name, 0, usage)

	f.callbacks = append(f.callbacks, makeApplier(f.cmd, name, ptr, &val))
}

// enumVar binds a string flag restricted to specific allowed values.
func enumVar[T ~string](binder *flagBinder, name string, usage string, target **T) {
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

func (f *flagBinder) bindAll() error {
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
