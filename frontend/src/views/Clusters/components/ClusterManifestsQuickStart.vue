<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import TIcon from '@/components/Icon/TIcon.vue'
import { getDocsLink } from '@/methods'
</script>

<template>
  <div class="@container flex grow flex-col items-center justify-center overflow-y-auto">
    <div class="flex flex-col items-center gap-3 text-center">
      <div class="flex size-14 items-center justify-center rounded-full bg-surface-raised">
        <TIcon icon="document-text" class="size-7 text-accent-text" />
      </div>

      <h1 class="text-xl font-medium text-content-emphasis">No Manifest Groups</h1>

      <p class="max-w-xl text-sm text-content-secondary">
        Omni tracks the Kubernetes bootstrap manifests it manages for this cluster — things like
        CNI, CSI, and other add-ons — and reports whether the cluster's live state matches what was
        applied. No manifest groups have been reported for this cluster yet.
      </p>

      <a
        :href="getDocsLink('omni', '/cluster-management/sync-kubernetes-manifests')"
        target="_blank"
        rel="noopener noreferrer"
        class="link-primary inline-flex items-center gap-1 text-sm"
      >
        Learn more about syncing Kubernetes manifests
        <TIcon icon="external-link" class="size-3.5" />
      </a>
    </div>

    <div class="grid max-w-3xl gap-3 py-6 @2xl:grid-cols-3">
      <div class="flex flex-col items-center gap-2 rounded-lg bg-surface-card p-4 text-center">
        <TIcon icon="pods" class="size-5 text-content-default" />
        <h3 class="text-sm font-medium text-content-emphasis">Grouped manifests</h3>
        <p class="text-xs text-content-secondary">
          Related Kubernetes objects are grouped so you can see the state of each bootstrap
          component at a glance.
        </p>
      </div>

      <div class="flex flex-col items-center gap-2 rounded-lg bg-surface-card p-4 text-center">
        <TIcon icon="check-in-circle" class="size-5 text-content-default" />
        <h3 class="text-sm font-medium text-content-emphasis">Sync tracking</h3>
        <p class="text-xs text-content-secondary">
          Each object is compared against the cluster's live state to detect drift from what Omni
          applied.
        </p>
      </div>

      <div class="flex flex-col items-center gap-2 rounded-lg bg-surface-card p-4 text-center">
        <TIcon icon="list-bullet" class="size-5 text-content-default" />
        <h3 class="text-sm font-medium text-content-emphasis">Graph &amp; list views</h3>
        <p class="text-xs text-content-secondary">
          Once available, manifests can be explored as a dependency graph or as a flat, filterable
          list.
        </p>
      </div>
    </div>
  </div>
</template>
