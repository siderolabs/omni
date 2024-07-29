// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package tests

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// AssertStatsLimits checks that metrics don't show any spikes of resource reads/writes, controller wakeups.
// This test should only be run after the integration tests set with Talemu enabled as the thresholds are adjusted for it.
// Should have Prometheus running on 9090.
func AssertStatsLimits(testCtx context.Context) TestFunc {
	return func(t *testing.T) {
		for _, tt := range []struct {
			check func(assert *assert.Assertions, value float64)
			name  string
			query string
		}{
			{
				name:  "resource CRUD",
				query: `sum(omni_resource_operations_total{operation=~"create|update", type!="MachineStatusLinks.omni.sidero.dev"})`,
				check: func(assert *assert.Assertions, value float64) { assert.Less(value, float64(10000)) },
			},
			{
				name:  "queue length",
				query: `sum(omni_runtime_qcontroller_queue_length)`,
				check: func(assert *assert.Assertions, value float64) { assert.Zero(value) },
			},
			{
				name:  "controller wakeups",
				query: `sum(omni_runtime_controller_wakeups{controller!="MachineStatusLinkController"})`,
				check: func(assert *assert.Assertions, value float64) { assert.Less(value, float64(10000)) },
			},
		} {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				ctx, cancel := context.WithTimeout(testCtx, time.Second*16)
				defer cancel()

				err := retry.Constant(time.Second * 15).Retry(func() error {
					promClient, err := api.NewClient(api.Config{
						Address: "http://127.0.0.1:9090",
					})
					if err != nil {
						return retry.ExpectedError(err)
					}

					var (
						value    model.Value
						warnings v1.Warnings
					)

					agg := assertionAggregator{}

					v1api := v1.NewAPI(promClient)

					value, warnings, err = v1api.Query(ctx, tt.query, time.Now())
					if err != nil {
						return retry.ExpectedError(err)
					}

					if len(warnings) > 0 {
						return retry.ExpectedErrorf("prometheus query had warnings %#v", warnings)
					}

					assert := assert.New(&agg)

					switch val := value.(type) {
					case *model.Scalar:
						tt.check(assert, float64(val.Value))
					case model.Vector:
						tt.check(assert, float64(val[val.Len()-1].Value))
					default:
						return fmt.Errorf("unexpected value type %s", val.Type())
					}

					if agg.hadErrors {
						return retry.ExpectedErrorf(agg.String())
					}

					return nil
				})

				require.NoError(t, err)
			})
		}
	}
}

type assertionAggregator struct {
	errors    map[string]struct{}
	hadErrors bool
}

func (agg *assertionAggregator) Errorf(format string, args ...any) {
	errorString := fmt.Sprintf(format, args...)

	if agg.errors == nil {
		agg.errors = map[string]struct{}{}
	}

	agg.errors[errorString] = struct{}{}
	agg.hadErrors = true
}

func (agg *assertionAggregator) String() string {
	lines := make([]string, 0, len(agg.errors))

	for errorString := range agg.errors {
		lines = append(lines, " * "+errorString)
	}

	sort.Strings(lines)

	return strings.Join(lines, "\n")
}
