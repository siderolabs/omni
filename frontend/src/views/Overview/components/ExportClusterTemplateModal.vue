<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { Runtime } from '@/api/common/omni.pb'
import type { ClusterSpec } from '@/api/omni/specs/omni.pb'
import { ClusterType, DefaultNamespace } from '@/api/resources'
import CodeBlock from '@/components/CodeBlock/CodeBlock.vue'
import ManagedByTemplatesWarning from '@/components/ManagedByTemplatesWarning.vue'
import Modal from '@/components/Modals/Modal.vue'
import { getDocsLink } from '@/methods'
import { useResourceWatch } from '@/methods/useResourceWatch'

const { clusterId } = defineProps<{
  clusterId: string
}>()

const open = defineModel<boolean>('open', { default: false })

const { data: cluster } = useResourceWatch<ClusterSpec>(() => ({
  skip: !open.value,
  runtime: Runtime.Omni,
  resource: {
    type: ClusterType,
    namespace: DefaultNamespace,
    id: clusterId,
  },
}))
</script>

<template>
  <Modal
    v-model:open="open"
    title="Export Cluster Template"
    cancel-label="Close"
    content-class="max-w-xl"
  >
    <template #description>Cluster {{ clusterId }}</template>

    <div class="flex flex-col gap-2 text-sm">
      <ManagedByTemplatesWarning
        :resource="cluster"
        warning-text-templates="This cluster is already managed using cluster templates. Make sure any external changes to templates have already been applied before exporting this template again, or your changes will be lost."
        warning-text-others="Make sure that any external changes to this cluster have already been applied before exporting the template, or your changes will be lost."
      />

      <p>
        You can export the
        <a
          class="link-primary"
          :href="getDocsLink('omni', '/reference/cluster-templates')"
          target="_blank"
          rel="noopener noreferrer"
        >
          cluster template
        </a>
        for this cluster using the following
        <code>omnictl</code>
        command
      </p>

      <CodeBlock :code="`omnictl cluster template export -c ${clusterId}`" />
    </div>
  </Modal>
</template>
