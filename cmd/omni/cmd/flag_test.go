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
	"github.com/siderolabs/omni/internal/pkg/config"
)

func TestFlagBinder(t *testing.T) {
	configSchema, err := config.ParseSchema()
	require.NoError(t, err)

	var (
		strFlag      *string
		boolFlag     *bool
		durationFlag *time.Duration
		intFlag      *int
		uint64Flag   *uint64
		uint32Flag   *uint32
		floatFlag    *float64
		unsetFlag    *string
	)

	type testEnum string

	var enumFlag *testEnum

	var flagBinder *cmd.FlagBinder

	command := &cobra.Command{
		Use: "test",
		RunE: func(cmd *cobra.Command, args []string) error {
			assert.Nil(t, strFlag, "flags should be nil before binding")
			assert.Nil(t, boolFlag, "flags should be nil before binding")
			assert.Nil(t, durationFlag, "flags should be nil before binding")
			assert.Nil(t, intFlag, "flags should be nil before binding")
			assert.Nil(t, uint64Flag, "flags should be nil before binding")
			assert.Nil(t, uint32Flag, "flags should be nil before binding")
			assert.Nil(t, floatFlag, "flags should be nil before binding")
			assert.Nil(t, enumFlag, "flags should be nil before binding")
			assert.Nil(t, unsetFlag, "flags should be nil before binding")

			require.NoError(t, flagBinder.BindAll())

			assert.Equal(t, "hello", *strFlag)
			assert.Equal(t, true, *boolFlag)
			assert.Equal(t, 90*time.Minute, *durationFlag)
			assert.Equal(t, 42, *intFlag)
			assert.Equal(t, uint64(123456789), *uint64Flag)
			assert.Equal(t, uint32(987654321), *uint32Flag)
			assert.InEpsilon(t, 3.14159, *floatFlag, 0.01)
			assert.Equal(t, testEnum("legacyAllowed"), *enumFlag)
			assert.Nil(t, unsetFlag, "unset flag should remain nil")

			return nil
		},
	}

	flagBinder = cmd.NewFlagBinder(command, configSchema)

	// Use real schema field paths to test that names and descriptions are derived correctly
	flagBinder.StringVar("account.id", &strFlag)
	flagBinder.BoolVar("auth.auth0.enabled", &boolFlag)
	flagBinder.DurationVar("auth.keyPruner.interval", &durationFlag)
	flagBinder.IntVar("services.siderolink.eventSinkPort", &intFlag)
	flagBinder.Uint64Var("etcdBackup.uploadLimitMbps", &uint64Flag)
	flagBinder.Uint32Var("auth.limits.maxUsers", &uint32Flag)
	flagBinder.Float64Var("logs.audit.cleanupProbability", &floatFlag)
	cmd.EnumVar(flagBinder, "services.siderolink.joinTokensMode", &enumFlag)
	flagBinder.StringVar("account.name", &unsetFlag)

	command.SetArgs([]string{
		"--account-id=hello",
		"--auth-auth0-enabled",
		"--public-key-pruning-interval=1h30m",
		"--event-sink-port=42",
		"--etcd-backup-upload-limit-mbps=123456789",
		"--auth-max-users=987654321",
		"--audit-log-cleanup-probability=3.14159",
		"--join-tokens-mode=legacyAllowed",
	})

	require.NoError(t, command.Execute())
}
