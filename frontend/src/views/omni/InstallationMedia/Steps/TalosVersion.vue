<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import TInput from '@/components/common/TInput/TInput.vue'
import type { FormState } from '@/views/omni/InstallationMedia/InstallationMediaCreate.vue'

const formState = defineModel<FormState>({ required: true })
</script>

<template>
  <div class="space-y-4">
    <!-- TODO: Select, get talos versions, default to ... ? -->
    <TInput v-model="formState.talosVersion" title="Choose Talos Linux Version" overhead-title />

    <p class="text-xs">
      We strongly recommend using the latest stable version of Talos Linux (1.10.4).
      <br />
      Pre-release versions are suitable for testing purposes but are not advised for production
      environments.
    </p>

    <div class="space-y-2">
      <h2 id="docs-label-id" class="text-xs font-medium text-naturals-n14 after:content-[':']">
        Documentation for Talos Linux 1.10.4
      </h2>
      <ul
        class="list-inside list-disc space-y-2 text-xs text-primary-p3"
        aria-labelledby="docs-label-id"
      >
        <li>
          <a
            href="https://docs.siderolabs.com/talos/v1.10/getting-started/what's-new-in-talos"
            rel="noopener noreferrer"
            target="_blank"
            class="underline"
          >
            What's New
          </a>
        </li>

        <li>
          <a
            href="https://docs.siderolabs.com/talos/v1.10/getting-started/support-matrix"
            rel="noopener noreferrer"
            target="_blank"
            class="underline"
          >
            Support Matrix
          </a>
        </li>
      </ul>
    </div>

    <!-- TODO: Select, get join tokens, default to ... ? -->
    <TInput v-model="formState.talosVersion" title="Join Token" overhead-title />
  </div>
</template>
