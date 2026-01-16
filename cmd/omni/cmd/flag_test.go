// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package cmd_test

import (
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/cmd/omni/cmd"
)

type TestEnum string

var (
	EnumValueA TestEnum = "valueA"
	EnumValueB TestEnum = "valueB"
)

func TestFlagBinder(t *testing.T) {
	var (
		strFlag      *string
		intFlag      *int
		boolFlag     *bool
		durationFlag *time.Duration
		uint64Flag   *uint64
		uint32Flag   *uint32
		floatFlag    *float64
		enumFlag     *TestEnum
		unsetFlag    *string
	)

	var flagBinder *cmd.FlagBinder

	command := &cobra.Command{
		Use: "test",
		RunE: func(cmd *cobra.Command, args []string) error {
			assert.Nil(t, strFlag, "flags should be nil before binding")
			assert.Nil(t, intFlag, "flags should be nil before binding")
			assert.Nil(t, boolFlag, "flags should be nil before binding")
			assert.Nil(t, durationFlag, "flags should be nil before binding")
			assert.Nil(t, uint64Flag, "flags should be nil before binding")
			assert.Nil(t, uint32Flag, "flags should be nil before binding")
			assert.Nil(t, floatFlag, "flags should be nil before binding")
			assert.Nil(t, enumFlag, "flags should be nil before binding")
			assert.Nil(t, unsetFlag, "flags should be nil before binding")

			require.NoError(t, flagBinder.BindAll())

			assert.Equal(t, "hello", *strFlag)
			assert.Equal(t, 42, *intFlag)
			assert.Equal(t, true, *boolFlag)
			assert.Equal(t, 90*time.Minute, *durationFlag)
			assert.Equal(t, uint64(123456789), *uint64Flag)
			assert.Equal(t, uint32(987654321), *uint32Flag)
			assert.InEpsilon(t, 3.14159, *floatFlag, 0.01)
			assert.Equal(t, EnumValueB, *enumFlag)
			assert.Nil(t, unsetFlag, "unset flag should remain nil")

			return nil
		},
	}

	flagBinder = cmd.NewFlagBinder(command)

	flagBinder.StringVar("str-flag", "A string flag", &strFlag)
	flagBinder.IntVar("int-flag", "An int flag", &intFlag)
	flagBinder.BoolVar("bool-flag", "A bool flag", &boolFlag)
	flagBinder.DurationVar("duration-flag", "A duration flag", &durationFlag)
	flagBinder.Uint64Var("uint64-flag", "A uint64 flag", &uint64Flag)
	flagBinder.Uint32Var("uint32-flag", "A uint32 flag", &uint32Flag)
	flagBinder.Float64Var("float-flag", "A float64 flag", &floatFlag)
	cmd.EnumVar(flagBinder, "enum-flag", "An enum flag", &enumFlag)
	flagBinder.StringVar("unset-flag", "An unset flag", &unsetFlag)

	command.SetArgs([]string{
		"--str-flag=hello",
		"--int-flag=42",
		"--bool-flag",
		"--duration-flag=1h30m",
		"--uint64-flag=123456789",
		"--uint32-flag=987654321",
		"--float-flag=3.14159",
		"--enum-flag=valueB",
	})

	require.NoError(t, command.Execute())
}
