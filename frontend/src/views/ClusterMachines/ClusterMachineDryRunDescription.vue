<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed } from 'vue'

import { type DryRunStatusSpec, DryRunStatusSpecOutcome } from '@/api/omni/specs/omni.pb'

import { dryRunSummary } from './dryRunSummary'

const { dryRun } = defineProps<{
  dryRun: DryRunStatusSpec
}>()

const summary = computed(() => dryRunSummary(dryRun))

const checkError = computed(() => dryRun.outcome !== DryRunStatusSpecOutcome.FAILED)
</script>

<template>
  <div class="flex max-w-md flex-col gap-tight">
    <p>{{ summary }}</p>
    <p class="text-content-muted">Nothing has been applied.</p>
    <template v-if="dryRun.error">
      <p v-if="checkError">The dry run could not run:</p>
      <p class="font-mono wrap-anywhere whitespace-pre-wrap">
        {{ dryRun.error }}
      </p>
    </template>
  </div>
</template>
