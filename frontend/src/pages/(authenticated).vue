<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<route lang="json">
{
  "meta": {
    "guard": "keys"
  }
}
</route>

<script setup lang="ts">
import { useDocumentTitle } from '@/methods/title'
import { useUserpilot } from '@/methods/userpilot'

useDocumentTitle()
useUserpilot()
</script>

<template>
  <RouterView />
</template>
