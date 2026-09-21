<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { useRoute } from 'vue-router'

import { useTitle } from '@/methods/title'
import MachineClass from '@/views/MachineClasses/MachineClass.vue'

definePage({ name: 'MachineClassEdit' })

const route = useRoute()

useTitle(() => ['Machine Classes', route.params.classname])
</script>

<template>
  <MachineClass :machine-class-edit-id="$route.params.classname" />
</template>
