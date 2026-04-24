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
  reboot: defineAsyncComponent(() => import('@/views/Modals/NodeReboot.vue')),
  shutdown: defineAsyncComponent(() => import('@/views/Modals/NodeShutdown.vue')),
  machineRemove: defineAsyncComponent(() => import('@/views/Modals/MachineRemove.vue')),
  machineClassDestroy: defineAsyncComponent(() => import('@/views/Modals/MachineClassDestroy.vue')),
  machineSetDestroy: defineAsyncComponent(() => import('@/views/Modals/MachineSetDestroy.vue')),
  maintenanceUpdate: defineAsyncComponent(() => import('@/views/Modals/MaintenanceUpdate.vue')),
  downloadOmnictlBinaries: defineAsyncComponent(() => import('@/views/Modals/DownloadOmnictl.vue')),
  exportClusterTemplate: defineAsyncComponent(
    () => import('@/views/Modals/ExportClusterTemplate.vue'),
  ),
  nodeDestroy: defineAsyncComponent(() => import('@/views/Modals/NodeDestroy.vue')),
  nodeDestroyCancel: defineAsyncComponent(() => import('@/views/Modals/NodeDestroyCancel.vue')),
  updateKubernetes: defineAsyncComponent(() => import('@/views/Modals/UpdateKubernetes.vue')),
  updateTalos: defineAsyncComponent(() => import('@/views/Modals/UpdateTalos.vue')),
  configPatchDestroy: defineAsyncComponent(() => import('@/views/Modals/ConfigPatchDestroy.vue')),
  userDestroy: defineAsyncComponent(() => import('@/views/Modals/UserDestroy.vue')),
  userCreate: defineAsyncComponent(() => import('@/views/Modals/UserCreate.vue')),
  updateKernelArgs: defineAsyncComponent(() => import('@/views/Modals/UpdateKernelArgs.vue')),
  joinTokenCreate: defineAsyncComponent(() => import('@/views/Modals/JoinTokenCreate.vue')),
  joinTokenRevoke: defineAsyncComponent(() => import('@/views/Modals/JoinTokenRevoke.vue')),
  joinTokenDelete: defineAsyncComponent(() => import('@/views/Modals/JoinTokenDelete.vue')),
  serviceAccountCreate: defineAsyncComponent(
    () => import('@/views/Modals/ServiceAccountCreate.vue'),
  ),
  serviceAccountRenew: defineAsyncComponent(() => import('@/views/Modals/ServiceAccountRenew.vue')),
  roleEdit: defineAsyncComponent(() => import('@/views/Modals/RoleEdit.vue')),
  infraProviderSetup: defineAsyncComponent(() => import('@/views/Modals/InfraProviderSetup.vue')),
  infraProviderDelete: defineAsyncComponent(() => import('@/views/Modals/InfraProviderDelete.vue')),
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
