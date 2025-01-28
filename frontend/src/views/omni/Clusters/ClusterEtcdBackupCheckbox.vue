<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="flex gap-1 items-center">
    <t-checkbox :checked="enabled"
        @click="toggleBackupsEnabled"
        label="Control Plane Backups"
        class="flex-1"
        :disabled="!backupStatus.enabled"
      />
    <div v-if="!backupStatus.enabled && backupStatus.configurable" class="text-xs flex gap-1 items-center">
      <t-button v-if="canManageBackupStore" type="subtle-xs" @click="$router.push({name: 'BackupStorage'})" icon="settings" icon-position="left" class="text-xs">Configure Storage</t-button>
      <div v-else class="truncate flex-1">Backup Storage Disabled</div>
    </div>
    <div class="flex text-xs items-center gap-2 h-6" v-else-if="enabled">
      <span>Interval:</span>
      <template v-if="!editingBackupConfig">
        <span class="text-naturals-N13">{{ interval?.toHuman() }}</span>
        <icon-button v-if="!editingBackupConfig" @click="startEditingBackupInterval" icon="edit"/>
      </template>
      <template v-else>
        <div class="w-12">
          <t-input
            @keydown.escape="editingBackupConfig = false"
            @keydown.enter="updateBackupInterval"
            v-click-outside="() => editingBackupConfig = false"
            type="number"
            v-model="backupIntervalPreview"
            compact
            focus
            :min="1"
            :max="24"
            />
        </div>
        <div class="text-naturals-N13">{{ pluralize('hour', backupIntervalPreview) }}</div>
        <icon-button @click="updateBackupInterval" icon="check"/>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, toRefs } from "vue";
import { Duration as GoogleProtobufDuration } from "@/api/google/protobuf/duration.pb";

import TCheckbox from "@/components/common/Checkbox/TCheckbox.vue";
import IconButton from "@/components/common/Button/IconButton.vue";
import { formatDuration, parseDuration } from "@/methods/time";
import pluralize from "pluralize";
import { Duration } from "luxon";
import TInput from "@/components/common/TInput/TInput.vue";
import { BackupsStatus } from "@/methods";
import TButton from "@/components/common/Button/TButton.vue";
import { canManageBackupStore } from "@/methods/auth";

const editingBackupConfig = ref(false);

const props = defineProps<{
  cluster: { backup_configuration?: { interval?: GoogleProtobufDuration, enabled?: boolean } },
  backupStatus: BackupsStatus
}>();

const { cluster } = toRefs(props);

const emit = defineEmits(['update:cluster']);

const enabled = computed(() => {
  return cluster.value.backup_configuration?.enabled === true;
});

const interval = computed(() => {
  const value = cluster.value.backup_configuration?.interval;

  if (!value) {
    return undefined;
  }

  return parseDuration(value).shiftTo("hours");
});

const backupIntervalPreview = ref(interval.value?.hours ?? 1);
const backupIntervalHours = ref(backupIntervalPreview.value);

const toggleBackupsEnabled = () => {
  const c = {...cluster.value};

  if (!c.backup_configuration) {
    c.backup_configuration = {};
  }

  c.backup_configuration.enabled = !c.backup_configuration.enabled;

  if (!c.backup_configuration.interval) {
    c.backup_configuration.interval = `${60 * 60}s`;
  }

  emit("update:cluster", c);
}

const startEditingBackupInterval = () => {
  backupIntervalPreview.value = interval.value?.hours ?? 1;
  backupIntervalHours.value = backupIntervalPreview.value;
  editingBackupConfig.value = true;
}

const updateBackupInterval = () => {
  backupIntervalHours.value = backupIntervalPreview.value;
  editingBackupConfig.value = false;

  const c = {...cluster.value};

  if (!c.backup_configuration) {
    c.backup_configuration = {};
  }

  c.backup_configuration.interval = formatDuration(Duration.fromDurationLike({ hours: backupIntervalHours.value }));

  emit("update:cluster", c);
};
</script>
