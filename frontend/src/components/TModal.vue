<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { type Component, defineAsyncComponent, type Ref, ref, shallowRef, watch } from 'vue'
import { useRoute } from 'vue-router'

import { modal } from '@/modal'

const modals: Record<string, Component> = {
  reboot: defineAsyncComponent(() => import('@/views/omni/Modals/NodeReboot.vue')),
  shutdown: defineAsyncComponent(() => import('@/views/omni/Modals/NodeShutdown.vue')),
  clusterDestroy: defineAsyncComponent(() => import('@/views/omni/Modals/ClusterDestroy.vue')),
  machineRemove: defineAsyncComponent(() => import('@/views/omni/Modals/MachineRemove.vue')),
  machineClassDestroy: defineAsyncComponent(
    () => import('@/views/omni/Modals/MachineClassDestroy.vue'),
  ),
  machineSetDestroy: defineAsyncComponent(
    () => import('@/views/omni/Modals/MachineSetDestroy.vue'),
  ),
  maintenanceUpdate: defineAsyncComponent(
    () => import('@/views/omni/Modals/MaintenanceUpdate.vue'),
  ),
  downloadInstallationMedia: defineAsyncComponent(
    () => import('@/views/omni/Modals/DownloadInstallationMedia.vue'),
  ),
  downloadOmnictlBinaries: defineAsyncComponent(
    () => import('@/views/omni/Modals/DownloadOmnictl.vue'),
  ),
  downloadSupportBundle: defineAsyncComponent(
    () => import('@/views/omni/Modals/DownloadSupportBundle.vue'),
  ),
  downloadTalosctlBinaries: defineAsyncComponent(
    () => import('@/views/omni/Modals/DownloadTalosctl.vue'),
  ),
  nodeDestroy: defineAsyncComponent(() => import('@/views/omni/Modals/NodeDestroy.vue')),
  nodeDestroyCancel: defineAsyncComponent(
    () => import('@/views/omni/Modals/NodeDestroyCancel.vue'),
  ),
  updateKubernetes: defineAsyncComponent(() => import('@/views/omni/Modals/UpdateKubernetes.vue')),
  updateTalos: defineAsyncComponent(() => import('@/views/omni/Modals/UpdateTalos.vue')),
  configPatchDestroy: defineAsyncComponent(
    () => import('@/views/omni/Modals/ConfigPatchDestroy.vue'),
  ),
  userDestroy: defineAsyncComponent(() => import('@/views/omni/Modals/UserDestroy.vue')),
  userCreate: defineAsyncComponent(() => import('@/views/omni/Modals/UserCreate.vue')),
  updateKernelArgs: defineAsyncComponent(() => import('@/views/omni/Modals/UpdateKernelArgs.vue')),
  joinTokenCreate: defineAsyncComponent(() => import('@/views/omni/Modals/JoinTokenCreate.vue')),
  joinTokenRevoke: defineAsyncComponent(() => import('@/views/omni/Modals/JoinTokenRevoke.vue')),
  joinTokenDelete: defineAsyncComponent(() => import('@/views/omni/Modals/JoinTokenDelete.vue')),
  serviceAccountCreate: defineAsyncComponent(
    () => import('@/views/omni/Modals/ServiceAccountCreate.vue'),
  ),
  serviceAccountRenew: defineAsyncComponent(
    () => import('@/views/omni/Modals/ServiceAccountRenew.vue'),
  ),
  roleEdit: defineAsyncComponent(() => import('@/views/omni/Modals/RoleEdit.vue')),
  machineAccept: defineAsyncComponent(() => import('@/views/omni/Modals/MachineAccept.vue')),
  machineReject: defineAsyncComponent(() => import('@/views/omni/Modals/MachineReject.vue')),
  updateExtensions: defineAsyncComponent(() => import('@/views/omni/Modals/UpdateExtensions.vue')),
  infraProviderSetup: defineAsyncComponent(
    () => import('@/views/omni/Modals/InfraProviderSetup.vue'),
  ),
  infraProviderDelete: defineAsyncComponent(
    () => import('@/views/omni/Modals/InfraProviderDelete.vue'),
  ),
}

const route = useRoute()
const view: Ref<Component | null> = shallowRef(null)
const props = ref({})

const updateState = () => {
  if (modal.value) {
    view.value = modal.value.component
    props.value = modal.value.props || {}

    return
  }

  props.value = {}
  view.value = route.query.modal ? modals[route.query.modal as string] : null
}

watch(
  () => route.query.modal,
  () => {
    if (route.query.modal) modal.value = null

    updateState()
  },
)

// modals which do not need to be tied to the URI
watch(modal, () => {
  updateState()
})

updateState()
</script>

<template>
  <div
    v-if="view"
    class="fixed top-0 right-0 bottom-0 left-0 z-30 flex items-center justify-center bg-naturals-n0/90 py-4"
  >
    <component :is="view" v-bind="props" />
  </div>
</template>
