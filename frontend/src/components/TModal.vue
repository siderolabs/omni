<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { modal } from '@/modal'
</script>

<template>
  <div
    v-if="modal?.component"
    class="fixed top-0 right-0 bottom-0 left-0 z-30 flex items-center justify-center bg-naturals-n0/90 py-4"
  >
    <component :is="modal.component" v-bind="modal.props ?? {}" />
  </div>
</template>
