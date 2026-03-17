<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import Confirmation from '@/views/omni/InstallationMedia/Confirmation.vue'
import type { FormState } from '@/views/omni/InstallationMedia/useFormState'

definePage({ name: 'InstallationMediaCreateConfirmation' })

const formState = defineModel<FormState>({ required: true })
</script>

<template>
  <Confirmation v-model="formState" />
</template>
