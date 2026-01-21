<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import {
  ExposedServiceIconAnnotationKey,
  ExposedServiceLabelAnnotationKey,
  ExposedServicePortAnnotationKey,
} from '@/api/resources'
import TCheckbox from '@/components/common/Checkbox/TCheckbox.vue'
import Tooltip from '@/components/common/Tooltip/Tooltip.vue'
import { useFeatures } from '@/methods/features'

type Props = {
  disabled?: boolean
}

defineProps<Props>()

const checked = defineModel<boolean>({ default: false })
const { data: features } = useFeatures()
</script>

<template>
  <Tooltip v-if="features?.spec.enable_workload_proxying" placement="bottom">
    <template #description>
      <div class="flex flex-col gap-1 p-2">
        <p>Enable HTTP proxying to the Services in the cluster through Omni.</p>
        <p>When enabled, the Services annotated with the following annotations</p>
        <p>will be listed and accessible from the Omni Web interface:</p>
        <div class="rounded bg-naturals-n5 px-2 py-1">
          <p class="font-mono">{{ ExposedServicePortAnnotationKey }} (required)</p>
          <p class="font-mono">{{ ExposedServiceLabelAnnotationKey }} (optional)</p>
          <p class="font-mono">{{ ExposedServiceIconAnnotationKey }} (optional)</p>
        </div>
        <p>
          If the icon is specified, it must be a valid base64 of either a gzipped or uncompressed
          svg image.
        </p>
      </div>
    </template>
    <TCheckbox v-model="checked" label="Workload Service Proxying" :disabled="disabled" />
  </Tooltip>
</template>
