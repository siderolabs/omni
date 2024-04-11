// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package resourcelogger_test

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/cosi-project/runtime/pkg/state/registry"
	"github.com/siderolabs/gen/xslices"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	"golang.org/x/sync/errgroup"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	resourceregistry "github.com/siderolabs/omni/client/pkg/omni/resources/registry"
	"github.com/siderolabs/omni/internal/backend/resourcelogger"
)

func TestResourceLogger(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	st := state.WrapCore(namespaced.NewState(inmem.Build))
	resourceRegistry := registry.NewResourceRegistry(st)

	for _, r := range resourceregistry.Resources {
		require.NoError(t, resourceRegistry.Register(ctx, r))
	}

	machineStatus := omni.NewMachineStatus(resources.DefaultNamespace, "test")
	cbs := omni.NewClusterBootstrapStatus(resources.DefaultNamespace, "test")
	cp := omni.NewConfigPatch(resources.DefaultNamespace, "test")

	logObserverCore, observedLogs := observer.New(zapcore.InfoLevel)

	logger := zap.New(logObserverCore)

	resLogger, err := resourcelogger.New(ctx, st, logger, zapcore.WarnLevel.String(), "machinestatus", "cbs", "configpatches")
	require.NoError(t, err)

	require.NoError(t, resLogger.StartWatches(ctx))

	var eg errgroup.Group

	t.Cleanup(func() {
		cancel()

		require.NoError(t, eg.Wait())
	})

	eg.Go(func() error { return resLogger.StartLogger(ctx) })

	machineStatus.TypedSpec().Value.Cluster = "some-cluster"
	machineStatus.TypedSpec().Value.ManagementAddress = "some-address"
	machineStatus.TypedSpec().Value.TalosVersion = "v1.2.3"
	machineStatus.TypedSpec().Value.Role = specs.MachineStatusSpec_WORKER
	machineStatus.TypedSpec().Value.Network = &specs.MachineStatusSpec_NetworkStatus{
		Hostname:        "aaa",
		Domainname:      "bbb",
		Addresses:       []string{"1.2.3.4", "5.6.7.8"},
		DefaultGateways: []string{"2.3.4.5"},
		NetworkLinks: []*specs.MachineStatusSpec_NetworkStatus_NetworkLinkStatus{
			{
				LinuxName:       "linux-name",
				HardwareAddress: "hw-address",
				SpeedMbps:       1234,
				LinkUp:          true,
				Description:     "hello",
			},
		},
	}
	cbs.TypedSpec().Value.Bootstrapped = false
	cp.TypedSpec().Value.Data = "some data"

	require.NoError(t, st.Create(ctx, machineStatus))
	require.NoError(t, st.Create(ctx, cbs))
	require.NoError(t, st.Create(ctx, cp))

	machineStatus.TypedSpec().Value.Cluster = "some-cluster-updated"
	machineStatus.TypedSpec().Value.ManagementAddress = "some-address-updated"
	machineStatus.TypedSpec().Value.Network.Domainname = "ccc"
	cbs.TypedSpec().Value.Bootstrapped = true
	cp.TypedSpec().Value.Data = "some data updated"

	require.NoError(t, st.Update(ctx, machineStatus))
	require.NoError(t, st.Update(ctx, cbs))
	require.NoError(t, st.Update(ctx, cp))

	require.NoError(t, st.Destroy(ctx, machineStatus.Metadata()))
	require.NoError(t, st.Destroy(ctx, cbs.Metadata()))
	require.NoError(t, st.Destroy(ctx, cp.Metadata()))

	assert.EventuallyWithT(t, func(*assert.CollectT) {
		entries := xslices.Map(observedLogs.All(), func(entry observer.LoggedEntry) logEntry {
			return parseLogEntry(t, entry)
		})

		assert.ElementsMatch(t, entries, []logEntry{
			{
				message:  "resource created",
				resource: "ClusterBootstrapStatuses.omni.sidero.dev(default/test@1)",
			},
			{
				message:  "resource created",
				resource: "MachineStatuses.omni.sidero.dev(default/test@1)",
			},
			{
				message:  "resource created",
				resource: "ConfigPatches.omni.sidero.dev(default/test@1)",
			},
			{
				message:  "resource updated",
				resource: "ClusterBootstrapStatuses.omni.sidero.dev(default/test@2)",
				diff: fmt.Sprintf(`--- ClusterBootstrapStatuses.omni.sidero.dev(default/test)
+++ ClusterBootstrapStatuses.omni.sidero.dev(default/test)
@@ -2,10 +2,10 @@
     namespace: default
     type: ClusterBootstrapStatuses.omni.sidero.dev
     id: test
-    version: 1
+    version: 2
     owner:
     phase: running
     created: %s
     updated: %s
 spec:
-    bootstrapped: false
+    bootstrapped: true
`, cbs.Metadata().Created().Format(time.RFC3339), cbs.Metadata().Updated().Format(time.RFC3339)),
			},
			{
				message:  "resource updated",
				resource: "MachineStatuses.omni.sidero.dev(default/test@2)",
				diff: fmt.Sprintf(`--- MachineStatuses.omni.sidero.dev(default/test)
+++ MachineStatuses.omni.sidero.dev(default/test)
@@ -2,7 +2,7 @@
     namespace: default
     type: MachineStatuses.omni.sidero.dev
     id: test
-    version: 1
+    version: 2
     owner:
     phase: running
     created: %s
@@ -12,7 +12,7 @@
     hardware: null
     network:
         hostname: aaa
-        domainname: bbb
+        domainname: ccc
         addresses:
             - 1.2.3.4
             - 5.6.7.8
@@ -25,10 +25,10 @@
               linkup: true
               description: hello
     lasterror: ""
-    managementaddress: some-address
+    managementaddress: some-address-updated
     connected: false
     maintenance: false
-    cluster: some-cluster
+    cluster: some-cluster-updated
     role: 2
     platformmetadata: null
     imagelabels: {}
`, machineStatus.Metadata().Created().Format(time.RFC3339)),
			},
			{
				message:  "resource updated",
				resource: "ConfigPatches.omni.sidero.dev(default/test@2)",
				// there should be no diff logged, as ConfigPatches are sensitive resources
			},
			{
				message:  "resource destroyed",
				resource: "MachineStatuses.omni.sidero.dev(default/test@2)",
			},
			{
				message:  "resource destroyed",
				resource: "ClusterBootstrapStatuses.omni.sidero.dev(default/test@2)",
			},
			{
				message:  "resource destroyed",
				resource: "ConfigPatches.omni.sidero.dev(default/test@2)",
			},
		})
	}, 5*time.Second, 100*time.Millisecond)
}

type logEntry struct {
	message  string
	resource string
	diff     string
}

func parseLogEntry(t *testing.T, entry observer.LoggedEntry) logEntry {
	require.NotEmpty(t, entry.Context)
	require.LessOrEqual(t, len(entry.Context), 2, "expected at most 2 context fields")
	require.Equal(t, "resource", entry.Context[0].Key)

	var diffLines []string

	if len(entry.Context) == 2 {
		require.Equal(t, "diff", entry.Context[1].Key)

		data := entry.Context[1].Interface

		// use reflection to convert private type zap.stringArray to []string

		require.Equal(t, reflect.Slice, reflect.TypeOf(data).Kind())
		require.Equal(t, reflect.String, reflect.TypeOf(data).Elem().Kind())

		val := reflect.ValueOf(data)

		diffLines = make([]string, 0, val.Len())

		for i := range val.Len() {
			diffLines = append(diffLines, val.Index(i).String())
		}
	}

	return logEntry{
		message:  entry.Message,
		resource: entry.Context[0].String,
		diff:     strings.Join(diffLines, "\n"),
	}
}
