<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { vOnClickOutside } from '@vueuse/components'
import { Duration } from 'luxon'
import pluralize from 'pluralize'
import { computed, ref } from 'vue'

import type { Duration as GoogleProtobufDuration } from '@/api/google/protobuf/duration.pb'
import IconButton from '@/components/Button/IconButton.vue'
import TButton from '@/components/Button/TButton.vue'
import TCheckbox from '@/components/Checkbox/TCheckbox.vue'
import TInput from '@/components/TInput/TInput.vue'
import type { BackupsStatus } from '@/methods'
import { usePermissions } from '@/methods/auth'
import { formatDuration, parseDuration } from '@/methods/time'

const editingBackupConfig = ref(false)

interface ClusterBackupConfiguration {
  backup_configuration?: {
    interval?: GoogleProtobufDuration
    enabled?: boolean
  }
}

interface Props {
  backupStatus: BackupsStatus
}

defineProps<Props>()

const cluster = defineModel<ClusterBackupConfiguration>('cluster', { required: true })

const { canManageBackupStore } = usePermissions()

const enabled = computed(() => {
  return cluster.value.backup_configuration?.enabled === true
})

const interval = computed(() => {
  const value = cluster.value.backup_configuration?.interval

  if (!value || value.trim() === '0') {
    return undefined
  }

  return parseDuration(value).shiftTo('hours')
})

const backupIntervalPreview = ref<number>()

const toggleBackupsEnabled = (enabled: boolean) => {
  cluster.value = {
    ...cluster.value,
    backup_configuration: {
      ...cluster.value.backup_configuration,
      interval: cluster.value.backup_configuration?.interval || `${60 * 60}s`,
      enabled,
    },
  }
}

const startEditingBackupInterval = () => {
  backupIntervalPreview.value = interval.value?.hours ?? 1
  editingBackupConfig.value = true
}

const updateBackupInterval = () => {
  editingBackupConfig.value = false

  cluster.value = {
    ...cluster.value,
    backup_configuration: {
      ...cluster.value.backup_configuration,
      interval: formatDuration(Duration.fromDurationLike({ hours: backupIntervalPreview.value })),
    },
  }
}
</script>

<template>
  <div class="flex items-center gap-1">
    <TCheckbox
      :model-value="enabled"
      label="Control Plane Backups"
      class="flex-1"
      :disabled="!backupStatus.enabled"
      @update:model-value="toggleBackupsEnabled"
    />
    <div
      v-if="!backupStatus.enabled && backupStatus.configurable"
      class="flex items-center gap-1 text-xs"
    >
      <TButton
        v-if="canManageBackupStore"
        variant="subtle"
        size="xxs"
        icon="settings"
        icon-position="left"
        class="text-xs"
        @click="$router.push({ name: 'BackupStorage' })"
      >
        Configure Storage
      </TButton>
      <div v-else class="flex-1 truncate">Backup Storage Disabled</div>
    </div>
    <div v-else-if="enabled" class="flex h-6 items-center gap-2 text-xs">
      <span>Interval:</span>
      <template v-if="!editingBackupConfig">
        <span class="text-naturals-n13">{{ interval?.toHuman() }}</span>
        <IconButton v-if="!editingBackupConfig" icon="edit" @click="startEditingBackupInterval" />
      </template>
      <template v-else>
        <div class="w-12">
          <TInput
            v-model="backupIntervalPreview"
            v-on-click-outside="() => (editingBackupConfig = false)"
            type="number"
            compact
            focus
            :min="1"
            :max="24"
            @keydown.escape="editingBackupConfig = false"
            @keydown.enter="updateBackupInterval"
          />
        </div>
        <div class="text-naturals-n13">{{ pluralize('hour', backupIntervalPreview) }}</div>
        <IconButton icon="check" @click="updateBackupInterval" />
      </template>
    </div>
  </div>
</template>
