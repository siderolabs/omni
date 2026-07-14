// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package omnictl

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"maps"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/siderolabs/omni/client/api/omni/management"
	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/omnictl/internal/access"
)

var auditLogFlags struct {
	search       string
	eventType    enumFlag[management.AuditLogEventType]
	orderByField enumFlag[management.AuditLogOrderByField]
	orderByDir   enumFlag[management.AuditLogOrderByDir]
	resourceType string
	resourceID   string
	clusterID    string
	actor        string
	since        time.Duration
	follow       bool
}

// auditLog represents audit-log command.
var auditLog = &cobra.Command{
	Use:   "audit-log [start] [end]",
	Short: "Read audit log from Omni",
	Long:  "Read audit log from Omni. Optionally filter by date range using start and end arguments in YYYY-MM-DD format (date-only, interpreted in the server's local time, e.g. 2024-01-01).",
	Args:  cobra.MaximumNArgs(2),
	RunE: func(_ *cobra.Command, arg []string) error {
		start := safeGet(arg, 0)
		end := safeGet(arg, 1)

		var startTsMs int64

		if auditLogFlags.since < 0 {
			return errors.New("--since must be positive")
		}

		if auditLogFlags.since > 0 {
			if start != "" {
				return errors.New("--since cannot be combined with the start argument")
			}

			startTsMs = time.Now().Add(-auditLogFlags.since).UnixMilli()
		}

		return access.WithClient(func(ctx context.Context, client *client.Client, _ access.ServerInfo) error {
			// the fields that cannot be combined with --follow are rejected by the server
			req := &management.ReadAuditLogRequest{
				StartTime:    start,
				EndTime:      end,
				StartTsMs:    startTsMs,
				Search:       auditLogFlags.search,
				EventType:    auditLogFlags.eventType.value,
				OrderByField: auditLogFlags.orderByField.value,
				OrderByDir:   auditLogFlags.orderByDir.value,
				ResourceType: auditLogFlags.resourceType,
				ResourceId:   auditLogFlags.resourceID,
				ClusterId:    auditLogFlags.clusterID,
				Actor:        auditLogFlags.actor,
			}

			var events iter.Seq2[*management.ReadAuditLogResponse, error]

			if auditLogFlags.follow {
				events = client.Management().FollowAuditLog(ctx, req)
			} else {
				events = client.Management().ReadAuditLog(ctx, req)
			}

			for resp, err := range events {
				if err != nil {
					// interrupting the command is not an error, e.g. ctrl-c on a followed stream
					if ctx.Err() != nil {
						return nil
					}

					return err
				}

				if _, err := os.Stdout.Write(resp.AuditLog); err != nil {
					return err
				}
			}

			return nil
		})
	},
}

// enumFlag is a pflag.Value implementation for proto enum types. It validates
// the input against a fixed set of allowed string values and stores the
// corresponding proto enum value.
type enumFlag[T ~int32] struct {
	value   T
	allowed map[string]T
}

// String implements pflag.Value.
func (f *enumFlag[T]) String() string {
	for k, v := range f.allowed {
		if v == f.value {
			return k
		}
	}

	return ""
}

// Set implements pflag.Value.
func (f *enumFlag[T]) Set(s string) error {
	v, ok := f.allowed[s]
	if !ok {
		return fmt.Errorf("must be one of: %s", strings.Join(slices.Sorted(maps.Keys(f.allowed)), ", "))
	}

	f.value = v

	return nil
}

// Type implements pflag.Value.
func (f *enumFlag[T]) Type() string {
	return strings.Join(slices.Sorted(maps.Keys(f.allowed)), "|")
}

func safeGet[T any](slc []T, pos int) T {
	if pos < len(slc) {
		return slc[pos]
	}

	return *new(T)
}

func init() {
	auditLogFlags.eventType = enumFlag[management.AuditLogEventType]{
		allowed: map[string]management.AuditLogEventType{
			"create":                management.AuditLogEventType_AUDIT_LOG_EVENT_TYPE_CREATE,
			"update":                management.AuditLogEventType_AUDIT_LOG_EVENT_TYPE_UPDATE,
			"update_with_conflicts": management.AuditLogEventType_AUDIT_LOG_EVENT_TYPE_UPDATE_WITH_CONFLICTS,
			"destroy":               management.AuditLogEventType_AUDIT_LOG_EVENT_TYPE_DESTROY,
			"teardown":              management.AuditLogEventType_AUDIT_LOG_EVENT_TYPE_TEARDOWN,
			"talos_access":          management.AuditLogEventType_AUDIT_LOG_EVENT_TYPE_TALOS_ACCESS,
			"k8s_access":            management.AuditLogEventType_AUDIT_LOG_EVENT_TYPE_K8S_ACCESS,
			"audit_log_access":      management.AuditLogEventType_AUDIT_LOG_EVENT_TYPE_AUDIT_LOG_ACCESS,
		},
	}

	auditLogFlags.orderByField = enumFlag[management.AuditLogOrderByField]{
		allowed: map[string]management.AuditLogOrderByField{
			"event_ts_ms":   management.AuditLogOrderByField_AUDIT_LOG_ORDER_BY_FIELD_DATE,
			"event_type":    management.AuditLogOrderByField_AUDIT_LOG_ORDER_BY_FIELD_EVENT_TYPE,
			"resource_type": management.AuditLogOrderByField_AUDIT_LOG_ORDER_BY_FIELD_RESOURCE_TYPE,
			"resource_id":   management.AuditLogOrderByField_AUDIT_LOG_ORDER_BY_FIELD_RESOURCE_ID,
			"cluster_id":    management.AuditLogOrderByField_AUDIT_LOG_ORDER_BY_FIELD_CLUSTER_ID,
			"actor":         management.AuditLogOrderByField_AUDIT_LOG_ORDER_BY_FIELD_ACTOR,
		},
	}

	auditLogFlags.orderByDir = enumFlag[management.AuditLogOrderByDir]{
		allowed: map[string]management.AuditLogOrderByDir{
			"asc":  management.AuditLogOrderByDir_AUDIT_LOG_ORDER_BY_DIR_ASC,
			"desc": management.AuditLogOrderByDir_AUDIT_LOG_ORDER_BY_DIR_DESC,
		},
	}

	auditLog.Flags().StringVar(&auditLogFlags.search, "search", "", "filter events by a search string")
	auditLog.Flags().Var(&auditLogFlags.eventType, "event-type", "filter events by event type")
	auditLog.Flags().Var(&auditLogFlags.orderByField, "order-by-field", "field to sort results by")
	auditLog.Flags().Var(&auditLogFlags.orderByDir, "order-by-dir", "sort direction")
	auditLog.Flags().StringVar(&auditLogFlags.resourceType, "resource-type", "", "filter events by resource type")
	auditLog.Flags().StringVar(&auditLogFlags.resourceID, "resource-id", "", "filter events by resource ID")
	auditLog.Flags().StringVar(&auditLogFlags.clusterID, "cluster-id", "", "filter events by cluster ID")
	auditLog.Flags().StringVar(&auditLogFlags.actor, "actor", "", "filter events by actor email")
	auditLog.Flags().BoolVarP(&auditLogFlags.follow, "follow", "f", false,
		"stream new events as they are written, starting at the current tail unless --since is given. "+
			"Cannot be combined with filters. The stream is re-established transparently whenever the "+
			"server ends it cleanly, while errors terminate the command.")
	auditLog.Flags().DurationVar(&auditLogFlags.since, "since", 0,
		"start from the given duration ago, e.g. 2h. More precise than the date-only start argument, "+
			"and the only way to include history in follow mode. A duration reaching past the "+
			"retention period returns everything still retained.")

	RootCmd.AddCommand(auditLog)
}
