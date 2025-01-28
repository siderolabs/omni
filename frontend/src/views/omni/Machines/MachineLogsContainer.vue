<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="flex flex-col">
    <div class="mb-4">
      <t-input
        placeholder="Search..."
        v-model="searchInput"
        icon="search"
      />
    </div>
    <log-viewer class="flex-1" :logs="logs" :searchOption="searchInput"/>
  </div>
</template>

<script setup lang="ts">
import { Ref, ref } from "vue";
import { useRoute } from "vue-router";
import { formatISO } from "@/methods/time";

import { setupLogStream, DefaultLogParser } from "@/methods/logs";
import { LogLine } from "@/methods/logs";
import { ManagementService } from "@/api/omni/management/management.pb";

import TInput from "@/components/common/TInput/TInput.vue";
import LogViewer from "@/components/common/LogViewer/LogViewer.vue";

const logs: Ref<LogLine[]> = ref([]);
const route = useRoute();

const searchInput = ref("");

interface Props {
  machineId?: string,
}

const props = defineProps<Props>()

setupLogStream(logs, ManagementService.MachineLogs, {
  machine_id: props.machineId || route.params.machine as string,
  tail_lines: -1,
  follow: true,
}, new DefaultLogParser((line: string): LogLine => {
  const data = JSON.parse(line);

  return {
    date: formatISO(data["talos-time"], "dd/MM/yyyy HH:mm:ss"),
    msg: data.msg,
  };
}))
</script>
