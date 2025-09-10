<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { useClipboard } from '@vueuse/core'
import { onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import type { Resource } from '@/api/grpc'
import { ResourceService } from '@/api/grpc'
import type { ClusterUUID } from '@/api/omni/specs/omni.pb'
import { withRuntime } from '@/api/options'
import { ClusterUUIDType, DefaultNamespace } from '@/api/resources'
import IconButton from '@/components/common/Button/IconButton.vue'
import TButton from '@/components/common/Button/TButton.vue'
import PageHeader from '@/components/common/PageHeader.vue'
import { setupBackupStatus } from '@/methods'
import { triggerEtcdBackup } from '@/methods/cluster'
import { showError } from '@/notification'
import BackupsList from '@/views/cluster/Backups/BackupsList.vue'

const { copy } = useClipboard()

const startingEtcdBackup = ref(false)
const route = useRoute()
const clusterUUID = ref('loading...')
const { status: backupStatus } = setupBackupStatus()

onMounted(async () => {
  const resp: Resource<ClusterUUID> = await ResourceService.Get(
    {
      id: route.params.cluster as string,
      namespace: DefaultNamespace,
      type: ClusterUUIDType,
    },
    withRuntime(Runtime.Omni),
  )

  clusterUUID.value = resp.spec.uuid!
})

const runEtcdBackup = async () => {
  startingEtcdBackup.value = true

  try {
    await triggerEtcdBackup(route.params.cluster as string)
  } catch (e) {
    showError('Failed to Trigger Manual Etcd Backup', e.message)
  }

  startingEtcdBackup.value = false
}
</script>

<template>
  <div class="flex flex-col">
    <div class="flex items-start gap-1">
      <PageHeader title="Control Plane Backups" class="flex-1" />
      <div class="flex items-center gap-1 text-xs">
        Cluster UUID:
        <div>{{ clusterUUID }}</div>
        <IconButton icon="copy" @click="() => copy(clusterUUID)" />
        <TButton
          type="highlighted"
          class="ml-2"
          :disabled="startingEtcdBackup || !backupStatus.enabled"
          @click="runEtcdBackup"
        >
          Trigger Etcd Backup
        </TButton>
      </div>
    </div>
    <BackupsList class="mb-6" />
  </div>
</template>
