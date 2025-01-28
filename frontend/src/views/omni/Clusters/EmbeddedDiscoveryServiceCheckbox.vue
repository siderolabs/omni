<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <tooltip placement="bottom">
    <template #description>
      <div class="flex flex-col gap-1 p-2">
        <p>Configure the cluster to use the discovery service embedded in Omni instead of the public one.</p>
        <p>This will only available if:</p>
        <p>- This Omni instance has the feature enabled.</p>
        <p>- Cluster's Talos version which supports connecting to it (>= v1.5.0).</p>
      </div>
    </template>
    <t-checkbox :checked="checked" :disabled="disabled" label="Use Embedded Discovery Service"/>
  </tooltip>
</template>

<script setup lang="ts">
import TCheckbox from "@/components/common/Checkbox/TCheckbox.vue";
import Tooltip from "@/components/common/Tooltip/Tooltip.vue";

type Props = {
  checked?: boolean;
  talosVersion?: string;
  disabled?: boolean;
};

defineProps<Props>();
</script>
