<!--
Copyright (c) 2025 Sidero Labs, Inc.

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
import { setupWorkloadProxyingEnabledFeatureWatch } from '@/methods/features'

type Props = {
  checked?: boolean
  disabled?: boolean
}

defineProps<Props>()

const workloadProxyingEnabled = setupWorkloadProxyingEnabledFeatureWatch()
</script>

<template>
  <Tooltip v-if="workloadProxyingEnabled" placement="bottom">
    <template #description>
      <div class="flex flex-col gap-1 p-2">
        <p>Enable HTTP proxying to the Services in the cluster through Omni.</p>
        <p>When enabled, the Services annotated with the following annotations</p>
        <p>will be listed and accessible from the Omni Web interface:</p>
        <div class="rounded bg-naturals-N5 px-2 py-1">
          <p class="font-roboto">{{ ExposedServicePortAnnotationKey }} (required)</p>
          <p class="font-roboto">{{ ExposedServiceLabelAnnotationKey }} (optional)</p>
          <p class="font-roboto">{{ ExposedServiceIconAnnotationKey }} (optional)</p>
        </div>
        <p>
          If the icon is specified, it must be a valid base64 of either a gzipped or uncompressed
          svg image.
        </p>
      </div>
    </template>
    <TCheckbox :checked="checked" label="Workload Service Proxying" :disabled="disabled" />
  </Tooltip>
</template>
