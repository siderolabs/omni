// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package router_test

import (
	"fmt"
	"maps"
	"slices"
	"testing"

	cosi "github.com/cosi-project/runtime/api/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/api"
	"github.com/siderolabs/talos/pkg/machinery/api/cluster"
	"github.com/siderolabs/talos/pkg/machinery/api/inspect"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/api/storage"
	"github.com/siderolabs/talos/pkg/machinery/api/time"
	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/omni/internal/backend/grpc/router"
)

// operatorMethods are the Talos API methods Omni forwards with os:operator at most, the rest is in adminMethodSet.
var operatorMethods = []string{
	// Talos accepts these from os:operator.
	cluster.ClusterService_HealthCheck_FullMethodName,
	cosi.State_Get_FullMethodName,
	cosi.State_List_FullMethodName,
	cosi.State_Watch_FullMethodName,
	inspect.InspectService_ControllerRuntimeDependencies_FullMethodName,
	machine.ImageService_List_FullMethodName,
	machine.ImageService_Pull_FullMethodName,
	machine.ImageService_Verify_FullMethodName,
	machine.MachineService_CPUFreqStats_FullMethodName,
	machine.MachineService_CPUInfo_FullMethodName,
	machine.MachineService_Containers_FullMethodName,
	machine.MachineService_DiskStats_FullMethodName,
	machine.MachineService_DiskUsage_FullMethodName,
	machine.MachineService_Dmesg_FullMethodName,
	machine.MachineService_EtcdAlarmDisarm_FullMethodName,
	machine.MachineService_EtcdAlarmList_FullMethodName,
	machine.MachineService_EtcdDefragment_FullMethodName,
	machine.MachineService_EtcdMemberList_FullMethodName,
	machine.MachineService_EtcdSnapshot_FullMethodName,
	machine.MachineService_EtcdStatus_FullMethodName,
	machine.MachineService_Events_FullMethodName,
	machine.MachineService_Hostname_FullMethodName,
	machine.MachineService_ImageList_FullMethodName,
	machine.MachineService_ImagePull_FullMethodName,
	machine.MachineService_List_FullMethodName,
	machine.MachineService_LoadAvg_FullMethodName,
	machine.MachineService_Logs_FullMethodName,
	machine.MachineService_LogsContainers_FullMethodName,
	machine.MachineService_Memory_FullMethodName,
	machine.MachineService_Mounts_FullMethodName,
	machine.MachineService_Netstat_FullMethodName,
	machine.MachineService_NetworkDeviceStats_FullMethodName,
	machine.MachineService_PacketCapture_FullMethodName,
	machine.MachineService_Processes_FullMethodName,
	machine.MachineService_Reboot_FullMethodName,
	machine.MachineService_Restart_FullMethodName,
	machine.MachineService_ServiceList_FullMethodName,
	machine.MachineService_ServiceRestart_FullMethodName,
	machine.MachineService_ServiceStart_FullMethodName,
	machine.MachineService_ServiceStop_FullMethodName,
	machine.MachineService_Shutdown_FullMethodName,
	machine.MachineService_Stats_FullMethodName,
	machine.MachineService_SystemStat_FullMethodName,
	machine.MachineService_Version_FullMethodName,
	machine.StorageService_Statfs_FullMethodName,
	storage.StorageService_Disks_FullMethodName,
	time.TimeService_Time_FullMethodName,
	time.TimeService_TimeCheck_FullMethodName,

	// Talos rejects these for os:operator, on purpose: Omni handles these flows itself.
	cosi.State_Create_FullMethodName,
	cosi.State_Destroy_FullMethodName,
	cosi.State_Teardown_FullMethodName,
	cosi.State_TeardownAndDestroy_FullMethodName,
	cosi.State_Update_FullMethodName,
	machine.ImageService_Import_FullMethodName,
	machine.LifecycleService_Install_FullMethodName,
	machine.LifecycleService_Upgrade_FullMethodName,
	machine.MachineService_ApplyConfiguration_FullMethodName,
	machine.MachineService_Bootstrap_FullMethodName,
	machine.MachineService_EtcdLeaveCluster_FullMethodName,
	machine.MachineService_EtcdRecover_FullMethodName,
	machine.MachineService_EtcdRemoveMemberByID_FullMethodName,
	machine.MachineService_GenerateClientConfiguration_FullMethodName,
	machine.MachineService_Kubeconfig_FullMethodName,
	machine.MachineService_Reset_FullMethodName,
	machine.MachineService_Rollback_FullMethodName,
	machine.MachineService_Upgrade_FullMethodName,
}

// TestTalosMethodsDecided fails when Talos serves an API method whose access Omni has not decided yet.
func TestTalosMethodsDecided(t *testing.T) {
	var talosMethods []string

	for _, fd := range api.TalosAPIdAllAPIs() {
		services := fd.Services()

		for i := range services.Len() {
			service := services.Get(i)
			methods := service.Methods()

			for j := range methods.Len() {
				talosMethods = append(talosMethods, fmt.Sprintf("/%s/%s", service.FullName(), methods.Get(j).Name()))
			}
		}
	}

	decidedMethods := slices.Concat(
		slices.Collect(maps.Keys(router.AdminMethodSet)),
		slices.Collect(maps.Keys(router.AdminMethodSet1_12)),
		operatorMethods,
	)

	missing := slices.DeleteFunc(slices.Clone(talosMethods), func(method string) bool { return slices.Contains(decidedMethods, method) })
	stale := slices.DeleteFunc(slices.Clone(decidedMethods), func(method string) bool { return slices.Contains(talosMethods, method) })

	assert.Empty(t, missing, "Talos added these API methods, decide whether Omni admins get os:admin for them and add them to adminMethodSet or operatorMethods")
	assert.Empty(t, stale, "Talos no longer serves these API methods")
}
