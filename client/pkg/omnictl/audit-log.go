// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package omnictl

import (
	"context"
	"os"

	"github.com/spf13/cobra"

	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/omnictl/internal/access"
)

// auditLog represents audit-log command.
var auditLog = &cobra.Command{
	Use:   "audit-log [start] [end]",
	Short: "Read audit log from Omni",
	Long:  "Read audit log from Omni. Optionally filter by date range using start and end arguments in YYYY-MM-DD format (date-only, interpreted in the server's local time, e.g. 2024-01-01).",
	Args:  cobra.MaximumNArgs(2),
	RunE: func(_ *cobra.Command, arg []string) error {
		start := safeGet(arg, 0)
		end := safeGet(arg, 1)

		return access.WithClient(func(ctx context.Context, client *client.Client, _ access.ServerInfo) error {
			for resp, err := range client.Management().ReadAuditLog(ctx, start, end) {
				if err != nil {
					return err
				}

				_, err := os.Stdout.Write(resp.AuditLog)
				if err != nil {
					return err
				}
			}

			return nil
		})
	},
}

func safeGet[T any](slc []T, pos int) T {
	if pos < len(slc) {
		return slc[pos]
	}

	return *new(T)
}

func init() {
	RootCmd.AddCommand(auditLog)
}
