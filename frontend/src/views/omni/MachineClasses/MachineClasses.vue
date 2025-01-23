<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="flex flex-col">
    <div class="flex gap-1 items-start">
      <page-header title="Machine Classes" class="flex-1"/>
      <t-button :disabled="!canRemoveMachines" @click="openMachineClassCreate" type="highlighted">Create Machine Class</t-button>
    </div>
    <machine-classes-list class="mb-6"/>
  </div>
</template>

<script setup lang="ts">
import PageHeader from "@/components/common/PageHeader.vue";
import TButton from "@/components/common/Button/TButton.vue";
import MachineClassesList from "@/views/omni/MachineClasses/MachineClassesList.vue";

import { canRemoveMachines } from "@/methods/auth";
import { useRouter } from "vue-router";

const router = useRouter();

const openMachineClassCreate = () => {
  router.push({ name: "MachineClassCreate" })
};
</script>
