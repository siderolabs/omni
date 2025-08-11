<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { useRouter } from 'vue-router'

import TButton from '@/components/common/Button/TButton.vue'
import PageHeader from '@/components/common/PageHeader.vue'
import { canRemoveMachines } from '@/methods/auth'
import MachineClassesList from '@/views/omni/MachineClasses/MachineClassesList.vue'

const router = useRouter()

const openMachineClassCreate = () => {
  router.push({ name: 'MachineClassCreate' })
}
</script>

<template>
  <div class="flex flex-col">
    <div class="flex items-start gap-1">
      <PageHeader title="Machine Classes" class="flex-1" />
      <TButton :disabled="!canRemoveMachines" type="highlighted" @click="openMachineClassCreate"
        >Create Machine Class</TButton
      >
    </div>
    <MachineClassesList class="mb-6" />
  </div>
</template>
