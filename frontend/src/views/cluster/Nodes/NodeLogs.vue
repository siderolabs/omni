<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="logs">
    <machine-logs-container
        v-if="$route.params.service === 'machine'" :machineId="route.params.machine as string"
        class="logs-container"
    />
    <div v-else class="logs-container">
      <div class="mb-4">
        <t-input
          placeholder="Search..."
          v-model="inputValue"
          icon="search"
        />
      </div>
      <t-alert
        v-if="err"
        :title="logs ? 'Disconnected' : 'Failed to Fetch Logs'"
        type="error"
        class="mb-2"
      >
        {{ err }}
      </t-alert>
      <log-viewer :logs="logs" :searchOption="inputValue" class="flex-1" :with-date="parsers[$route.params.service as string] !== undefined"/>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, Ref, watch } from "vue";
import { useRoute } from "vue-router";
import { LogsRequest, MachineService } from "@/api/talos/machine/machine.pb";
import { Runtime } from "@/api/common/omni.pb";
import { getContext } from "@/context";
import { setupLogStream, LogLine, LineDelimitedLogParser } from "@/methods/logs";
import { withContext, withRuntime } from "@/api/options";
import { DateTime } from "luxon";

import TInput from "@/components/common/TInput/TInput.vue";
import TAlert from "@/components/TAlert.vue";
import LogViewer from "@/components/common/LogViewer/LogViewer.vue";
import MachineLogsContainer from "@/views/omni/Machines/MachineLogsContainer.vue";

const route = useRoute();
const inputValue = ref("");
const logs: Ref<LogLine[]> = ref([]);
const context = getContext();
const service = ref(route.params.service as string);

const formatLoggingContext = (logRecord: Record<string, string>, ...exceptFields: string[]) => {
  const res: string[] = [];

  for (const key in logRecord) {
    if (exceptFields.includes(key)) {
      continue;
    }

    res.push(`${key}=${logRecord[key]}`);
  }

  return res.join(" ");
}

const parsers = {
  containerd: (line: string): LogLine => {
    const parsed = JSON.parse(line);

    return {
      date: parsed.time,
      msg: `[${parsed.level ?? 'info'}] ${parsed.msg} ${formatLoggingContext(parsed, "msg", "ts", "level")}`,
    }
  },
  cri: (line: string): LogLine => {
    const parsed = JSON.parse(line);

    return {
      date: parsed.time,
      msg: `[${parsed.level ?? 'info'}] ${parsed.msg} ${formatLoggingContext(parsed, "msg", "ts", "level")}`,
    }
  },
  etcd: (line: string): LogLine => {
    const parsed = JSON.parse(line);

    return {
      date: parsed.ts,
      msg: `[${parsed.level ?? 'info'}] ${parsed.msg} ${formatLoggingContext(parsed, "msg", "ts", "level")}`,
    }
  },
  kubelet: (line: string): LogLine => {
    const parsed = JSON.parse(line);
    const date = DateTime.fromSeconds(parseFloat(parsed.ts) / 1000);

    return {
      date: date.toISO(),
      msg: `[${parsed.level ?? 'info'}] ${parsed.msg} ${formatLoggingContext(parsed, "msg", "ts", "level")}`,
    }
  },
};

const plainText = (line: string) => {
  return {
    msg: line
  };
}

const params = computed<LogsRequest>(() => {
  return {
    namespace: "system",
    id: service.value,
    follow: true,
    tail_lines: -1,
  };
});

const getLineParser = (svc: string) => {
  return parsers[svc] ? (line: string): LogLine => {
    try {
      return parsers[svc](line);
    } catch {
      return plainText(line);
    }
  } : plainText;
}

const logParser = new LineDelimitedLogParser(getLineParser(service.value));

watch(() => route.params.service, () => {
  const svc = route.params.service as string;

  logParser.setLineParser(getLineParser(svc));
  service.value = svc;
})

const stream = setupLogStream(logs, MachineService.Logs, params, logParser, withRuntime(Runtime.Talos), withContext(context));

const err = computed(() => {
  return stream.value?.err;
});
</script>

<style scoped>
.logs {
  @apply flex flex-col h-full;
  max-height: calc(100vh - 150px);
  overflow: hidden;
}
.logs-container {
  @apply flex flex-col;
  flex-grow: 1;
}
</style>
