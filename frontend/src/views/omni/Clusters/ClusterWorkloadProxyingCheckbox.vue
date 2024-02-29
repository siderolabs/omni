<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <tooltip placement="bottom" v-if="workloadProxyingEnabled">
    <template #description>
      <div class="flex flex-col gap-1 p-2">
        <p>Enable HTTP proxying to the Services in the cluster through Omni.</p>
        <p>When enabled, the Services annotated with the following annotations</p>
        <p>will be listed and accessible from the Omni Web interface:</p>
        <div class="bg-naturals-N5 rounded px-2 py-1">
          <p class="font-roboto">{{ ServicePortAnnotationKey }} (required)</p>
          <p class="font-roboto">{{ ServiceLabelAnnotationKey }} (optional)</p>
          <p class="font-roboto">{{ ServiceIconAnnotationKey }} (optional)</p>
        </div>
        <p>If the icon is specified, it must be a valid base64 of either a gzipped or uncompressed svg image.</p>
      </div>
    </template>
    <t-checkbox :checked="checked" label="Workload Service Proxying" :disabled="disabled"/>
  </tooltip>
</template>

<script setup lang="ts">
import { setupWorkloadProxyingEnabledFeatureWatch } from "@/methods/features";
import TCheckbox from "@/components/common/Checkbox/TCheckbox.vue";
import Tooltip from "@/components/common/Tooltip/Tooltip.vue";
import { ServiceIconAnnotationKey, ServiceLabelAnnotationKey, ServicePortAnnotationKey } from "@/api/resources";

type Props = {
  checked?: boolean;
  disabled?: boolean;
};

defineProps<Props>();

const workloadProxyingEnabled = setupWorkloadProxyingEnabledFeatureWatch();
</script>
