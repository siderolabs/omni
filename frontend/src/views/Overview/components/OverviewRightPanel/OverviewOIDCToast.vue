<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { getDocsLink } from '@/methods'

const oidcDocsLink = getDocsLink(
  'omni',
  '/getting-started/use-kubectl-with-omni#install-the-oidc-plug-in',
)
</script>

<template>
  You will need to install the
  <a class="link-primary" :href="oidcDocsLink" target="_blank" rel="noopener noreferrer">
    OIDC plugin
  </a>
  when using the downloaded kubeconfig in kubectl
</template>
