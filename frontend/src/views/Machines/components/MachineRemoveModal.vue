<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import pluralize from 'pluralize'
import { ref, watchEffect } from 'vue'

import ConfirmModal from '@/components/Modals/ConfirmModal.vue'
import { removeMachine } from '@/methods/machine'
import { showError, showSuccess } from '@/notification'

const { clusters, machines } = defineProps<{
  clusters: string[]
  machines: string[]
}>()

const open = defineModel<boolean>('open', { default: false })

const isRemoving = ref(false)

watchEffect(() => {
  if (open.value) return

  isRemoving.value = false
})

const remove = async () => {
  isRemoving.value = true

  try {
    await Promise.all(machines.map(removeMachine))

    showSuccess(
      `The ${pluralize('machine', machines.length)} "${machines.join('", "')}" ${pluralize('was', machines.length)} removed`,
    )

    open.value = false
  } catch (e) {
    showError(
      `Failed to remove the ${pluralize('machine', machines.length)}`,
      e instanceof Error ? e.message : String(e),
    )
  } finally {
    isRemoving.value = false
  }
}
</script>

<template>
  <ConfirmModal
    v-model:open="open"
    :title="`Destroy ${pluralize('machine', machines.length, true)}`"
    :loading="isRemoving"
    action-label="Remove"
    content-class="max-w-xl"
    @confirm="remove"
  >
    <div class="flex flex-col gap-4 text-xs">
      <ul class="list-inside list-disc text-xs">
        <li v-for="machine in machines" :key="machine">
          <code>{{ machine }}</code>
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
          Destroying the {{ pluralize('machine', machines.length) }} should be only used as a last
          resort, e.g. in a case of a hardware failure.
        </p>

        <p class="font-bold text-primary-p3">
          The {{ pluralize('machine', machines.length) }} will need to be wiped and reinstalled to
          be used again with Omni.
        </p>

        <p>
          If you want to remove the {{ pluralize('machine', machines.length) }} from the
          {{ pluralize('clusters', clusters.length) }}, please use the
          <RouterLink :to="{ name: 'ClusterOverview', params: { cluster: clusters[0] } }">
            cluster overview page
          </RouterLink>
          .
        </p>
      </template>
    </div>
  </ConfirmModal>
</template>
