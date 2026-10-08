<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed, ref } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import type { KubernetesHealthCheckStatusSpec } from '@/api/omni/specs/omni.pb'
import { KubernetesHealthCheckStatusSpecState } from '@/api/omni/specs/omni.pb'
import { EphemeralNamespace, KubernetesHealthCheckStatusType, LabelCluster } from '@/api/resources'
import TButton from '@/components/Button/TButton.vue'
import CodeBlock from '@/components/CodeBlock/CodeBlock.vue'
import type { IconType } from '@/components/Icon/TIcon.vue'
import TIcon from '@/components/Icon/TIcon.vue'
import LogViewer from '@/components/LogViewer/LogViewer.vue'
import Modal from '@/components/Modals/Modal.vue'
import TInput from '@/components/TInput/TInput.vue'
import type { LogLine } from '@/methods/logs'
import { useResourceWatch } from '@/methods/useResourceWatch'

const { clusterId } = defineProps<{ clusterId: string }>()

const { data: healthChecks } = useResourceWatch<KubernetesHealthCheckStatusSpec>(() => ({
  runtime: Runtime.Omni,
  resource: {
    namespace: EphemeralNamespace,
    type: KubernetesHealthCheckStatusType,
  },
  selectors: [`${LabelCluster}=${clusterId}`],
}))

const State = KubernetesHealthCheckStatusSpecState

const ICON_STATES: Record<KubernetesHealthCheckStatusSpecState, { icon: IconType; class: string }> =
  {
    [State.PASSED]: { icon: 'check-circle', class: 'text-status-success-default' },
    [State.FAILED]: { icon: 'x-circle', class: 'text-status-danger-default' },
    [State.RUNNING]: { icon: 'loading', class: 'animate-spin text-status-warning-default' },
    [State.UNKNOWN]: { icon: 'time', class: 'text-content-muted' },
  }

const iconForStatus = ({ state = State.UNKNOWN }) => ICON_STATES[state]

// the healthcheck whose output is shown in the modal. The ID is kept while the modal animates closed,
// so its content does not flash as it disappears
const checkModal = ref<{ id?: string; open: boolean }>({ open: false })
const searchOutput = ref('')

const openedCheck = computed(() =>
  healthChecks.value.find((check) => check.metadata.id === checkModal.value.id),
)

// the output is the tail of the failing container's own output, captured by Omni before the runner
// job was deleted - it carries no timestamps of its own
const outputLines = computed<LogLine[]>(() =>
  (openedCheck.value?.spec.output ?? '').split('\n').map((msg) => ({ msg })),
)

const kubectlLogs = (spec: KubernetesHealthCheckStatusSpec) =>
  `kubectl logs -n ${spec.job_namespace} job/${spec.job_name}`
</script>

<template>
  <div
    v-if="healthChecks.length"
    class="mb-5 rounded border border-border-default bg-surface-card pt-5"
  >
    <div class="flex items-center gap-micro px-base pb-compact">
      <span class="flex-1 text-sm text-content-default">Health Checks</span>
    </div>

    <div class="flex flex-col gap-tight border-t-8 border-border-default p-compact text-xs">
      <div
        v-for="check in healthChecks"
        :key="check.metadata.id"
        class="flex min-w-0 items-center gap-tight"
      >
        <TIcon
          :icon="iconForStatus(check.spec).icon"
          :class="iconForStatus(check.spec).class"
          class="h-6 w-6 shrink-0"
        />
        <span class="min-w-0 flex-1 truncate text-content-default">{{ check.metadata.id }}</span>
        <TButton
          v-if="check.spec.output"
          variant="secondary"
          icon="log"
          class="shrink-0"
          @click="checkModal = { open: true, id: check.metadata.id }"
        >
          View output
        </TButton>
      </div>
    </div>

    <Modal
      v-model:open="checkModal.open"
      :title="`Health Check: ${checkModal.id}`"
      cancel-label="Close"
      content-class="flex max-w-3xl flex-col gap-compact overflow-hidden"
    >
      <template #description>
        Output of the last failed run. Omni captures it before deleting the runner job, so it stays
        available after the job's pod is gone.
      </template>

      <div v-if="openedCheck?.spec.state === State.RUNNING" class="flex flex-col gap-micro">
        <span class="text-xs text-content-muted">Follow a running check with:</span>
        <CodeBlock
          :button-attrs="{ 'aria-label': 'Copy kubectl logs command' }"
          :code="kubectlLogs(openedCheck.spec)"
        />
      </div>

      <TInput v-model="searchOutput" placeholder="Search..." icon="search" />

      <div
        class="flex h-96 min-h-0 flex-col overflow-hidden rounded border border-border-default bg-surface-card"
      >
        <LogViewer
          class="min-h-0 grow"
          :logs="outputLines"
          :search-option="searchOutput"
          without-date
        />
      </div>
    </Modal>
  </div>
</template>
