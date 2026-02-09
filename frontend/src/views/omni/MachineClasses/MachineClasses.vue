<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import TButton from '@/components/common/Button/TButton.vue'
import PageHeader from '@/components/common/PageHeader.vue'
import { canRemoveMachines } from '@/methods/auth'
import MachineClassesList from '@/views/omni/MachineClasses/MachineClassesList.vue'
</script>

<template>
  <div class="flex h-full flex-col">
    <div class="flex items-start gap-1">
      <PageHeader title="Machine Classes" class="flex-1" />
      <TButton
        is="router-link"
        :disabled="!canRemoveMachines"
        variant="highlighted"
        :to="{ name: 'MachineClassCreate' }"
      >
        Create Machine Class
      </TButton>
    </div>
    <MachineClassesList class="mb-6" />
  </div>
</template>
