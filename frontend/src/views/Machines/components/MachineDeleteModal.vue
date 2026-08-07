<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import pluralize from 'pluralize'
import { ref, watchEffect } from 'vue'

import ConfirmModal from '@/components/Modals/ConfirmModal.vue'
import { deleteMachine } from '@/methods/machine'
import { showError, showSuccess } from '@/notification'

const { clusters = [], machines } = defineProps<{
  clusters?: string[]
  machines: { id: string; name?: string }[]
}>()

const open = defineModel<boolean>('open', { default: false })
const emit = defineEmits<{
  deleted: [machineIds: string[]]
}>()

const isDeleting = ref(false)

watchEffect(() => {
  if (open.value) return

  isDeleting.value = false
})

async function onConfirm() {
  isDeleting.value = true

  const result = (await Promise.allSettled(machines.map(({ id }) => deleteMachine(id)))).map(
    (r, i) => ({
      ...r,
      machineId: machines[i].id,
    }),
  )

  const passes = result.filter((r) => r.status === 'fulfilled')
  const fails = result.filter((r) => r.status === 'rejected')

  if (passes.length) {
    showSuccess(`Deleted ${pluralize('machine', passes.length, passes.length > 1)}`)
  }

  if (fails.length) {
    showError(
      `Failed to delete ${pluralize('machine', fails.length, fails.length > 1)}`,
      fails
        .map((f) => (f.reason instanceof Error ? f.reason.message : String(f.reason)))
        .join('\n'),
    )
  } else {
    open.value = false
  }

  isDeleting.value = false

  emit(
    'deleted',
    passes.map((p) => p.machineId),
  )
}
</script>

<template>
  <ConfirmModal
    v-model:open="open"
    :title="`Delete ${pluralize('Machine', machines.length, machines.length !== 1)}`"
    :loading="isDeleting"
    action-label="Delete"
    content-class="max-w-xl"
    @confirm="onConfirm"
  >
    <div class="flex flex-col gap-4 text-xs">
      <ul class="list-inside list-disc">
        <li v-for="machine in machines" :key="machine.id">
          <code>{{ machine.name ?? machine.id }}</code>
        </li>
      </ul>

      <p>Please confirm the action.</p>

      <template v-if="clusters.length">
        <p class="text-primary-p3">
          The {{ pluralize('machine', machines.length) }}
          {{ pluralize('is', machines.length) }} part of the
          {{ pluralize('clusters', clusters.length) }}
          <code
            v-for="cluster in clusters"
            :key="cluster"
            class="not-last-of-type:after:content-[','] last-of-type:after:content-['.']"
          >
            {{ cluster }}
          </code>
          Deleting the {{ pluralize('machine', machines.length) }} should be only used as a last
          resort, e.g. in a case of a hardware failure.
        </p>

        <p class="font-bold text-primary-p3">
          The {{ pluralize('machine', machines.length) }} will need to be wiped and reinstalled to
          be used again with Omni.
        </p>

        <p>
          If you want to remove the {{ pluralize('machine', machines.length) }} from the
          {{ pluralize('clusters', clusters.length) }}, please use the
          <RouterLink
            class="link-primary"
            :to="{ name: 'ClusterOverview', params: { cluster: clusters[0] } }"
          >
            Cluster Overview
          </RouterLink>
          page.
        </p>
      </template>

      <p v-else class="text-primary-p2">
        The {{ pluralize('machine', machines.length) }} will be deleted from Omni.
      </p>
    </div>
  </ConfirmModal>
</template>
