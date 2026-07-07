<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import TCheckbox from '@/components/Checkbox/TCheckbox.vue'
import Tooltip from '@/components/Tooltip/Tooltip.vue'

type Props = {
  disabled?: boolean
}

defineProps<Props>()

const checked = defineModel<boolean>({ default: false })
</script>

<template>
  <Tooltip placement="bottom">
    <template #description>
      <div class="flex flex-col gap-1 p-2">
        <p>
          Allow the omni.sidero.dev/node-audit-skip annotation on Kubernetes nodes to exempt them
          from Omni's node audit.
        </p>
        <p>
          Use this for virtual nodes (e.g. VirtualKubelet) that have no corresponding ClusterMachine
          and would otherwise be deleted.
        </p>
      </div>
    </template>
    <TCheckbox v-model="checked" :disabled="disabled" label="Enable Node Audit Skip" />
  </Tooltip>
</template>
