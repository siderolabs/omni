<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import pluralize from 'pluralize'
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import TButton from '@/components/common/Button/TButton.vue'
import { removeMachine } from '@/methods/machine'
import { showError, showSuccess } from '@/notification'
import CloseButton from '@/views/omni/Modals/CloseButton.vue'

const router = useRouter()
const route = useRoute()
const removing = ref(false)

const machines = computed(() => {
  const qMachine = route.query.machine

  const machines = Array.isArray(qMachine) ? qMachine : [qMachine]

  return machines.filter((m): m is string => !!m)
})

const clusters = computed(() => {
  const qCluster = route.query.cluster

  const clusters = Array.isArray(qCluster) ? qCluster : [qCluster]

  return clusters.filter((c): c is string => !!c)
})

const close = () => {
  router.back()
}

const remove = async () => {
  removing.value = true

  try {
    await Promise.all(machines.value.map(removeMachine))

    showSuccess(
      `The ${pluralize('machine', machines.value.length)} "${machines.value.join('", "')}" ${pluralize('was', machines.value.length)} removed`,
    )
  } catch (e) {
    showError(`Failed to remove the ${pluralize('machine', machines.value.length)}`, e.message)
  } finally {
    close()
  }
}
</script>

<template>
  <div class="modal-window flex flex-col gap-4 text-xs">
    <div class="flex items-center justify-between text-naturals-n14">
      <h3 class="grow text-base text-naturals-n14">
        Destroy {{ pluralize('machine', machines.length, true) }}
      </h3>

      <CloseButton class="shrink-0" @click.once="close" />
    </div>

    <ul class="list-inside list-disc">
      <li v-for="machine in machines" :key="machine">
        <code>{{ machine }}</code>
      </li>
    </ul>

    <p>Please confirm the action.</p>

    <div v-if="clusters.length" class="flex flex-col gap-4">
      <p class="text-primary-p3">
        The {{ pluralize('machine', machines.length) }} {{ pluralize('is', machines.length) }} part
        of the {{ pluralize('clusters', clusters.length) }}
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
        The {{ pluralize('machine', machines.length) }} will need to wiped and reinstalled to be
        used again with Omni.
      </p>

      <p>
        If you want to remove the {{ pluralize('machine', machines.length) }} from the
        {{ pluralize('clusters', clusters.length) }}, please use the
        <RouterLink :to="{ name: 'ClusterOverview', params: { cluster: clusters[0] } }">
          cluster overview page
        </RouterLink>
        .
      </p>
    </div>

    <TButton
      class="h-9 w-32 self-end"
      icon="delete"
      icon-position="left"
      :disabled="removing"
      @click.once="remove"
    >
      Remove
    </TButton>
  </div>
</template>
