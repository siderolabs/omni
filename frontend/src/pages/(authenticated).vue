<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { usePostHog } from '@/methods/posthog'
import { useDocumentTitle } from '@/methods/title'
import { useUserpilot } from '@/methods/userpilot'

definePage({
  meta: { guard: 'keys' },
})

useDocumentTitle()
useUserpilot()
usePostHog()
</script>

<template>
  <RouterView />
</template>
