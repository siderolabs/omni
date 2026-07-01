<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { RadioGroup, RadioGroupLabel } from '@headlessui/vue'
import { ref, watchEffect } from 'vue'

import TIcon from '@/components/Icon/TIcon.vue'
import ConfirmModal from '@/components/Modals/ConfirmModal.vue'
import RadioGroupOption from '@/components/Radio/RadioGroupOption.vue'
import { getDocsLink } from '@/methods'

const { talosVersion } = defineProps<{
  talosVersion?: string
}>()

const open = defineModel<boolean>('open', { default: false })

defineEmits<{
  continue: [untaint: boolean]
}>()

const untaint = ref(true)

watchEffect(() => {
  if (open.value) return

  untaint.value = true
})
</script>

<template>
  <ConfirmModal
    v-model:open="open"
    title="Untaint single node cluster"
    action-label="Create Cluster"
    content-class="max-w-xl"
    @confirm="
      () => {
        open = false
        $emit('continue', untaint)
      }
    "
  >
    <div class="flex flex-col gap-4">
      <p class="text-sm">
        This cluster has a single control plane node and no workers. By default, control plane nodes
        are tainted so that user workloads are not scheduled on them, which would leave this cluster
        unable to run any workloads.
      </p>

      <RadioGroup v-model="untaint">
        <RadioGroupLabel class="mb-3 block text-sm">
          Apply a patch (
          <code class="text-xs text-naturals-n13">allowSchedulingOnControlPlanes: true</code>
          ) that will enable scheduling user workloads on this node?
        </RadioGroupLabel>

        <div class="flex items-center gap-4">
          <RadioGroupOption :value="false">No</RadioGroupOption>
          <RadioGroupOption :value="true">Yes</RadioGroupOption>
        </div>
      </RadioGroup>

      <a
        class="link-primary inline-flex items-center gap-1 self-end text-xs"
        :href="
          getDocsLink('talos', '/deploy-and-manage-workloads/workers-on-controlplane', {
            talosVersion,
          })
        "
        target="_blank"
        rel="noopener noreferrer"
      >
        Enable workloads on your control plane nodes
        <TIcon icon="external-link" class="size-4" />
      </a>
    </div>
  </ConfirmModal>
</template>
