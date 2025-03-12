<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="modal-window">
    <div class="heading">
      <h3 class="text-base text-naturals-N14 truncate flex-1">
        Revoke the token {{ id }} ?
      </h3>
      <close-button @click="close" />
    </div>
    <p class="text-xs">Please confirm the action.</p>

    <div class="flex justify-end gap-4 mt-8">
      <t-button @click="revoke" class="w-32 h-9">
        Revoke
      </t-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useRoute, useRouter } from "vue-router";

import CloseButton from "@/views/omni/Modals/CloseButton.vue";
import TButton from "@/components/common/Button/TButton.vue";
import { revokeJoinToken } from "@/methods/auth";
import { showError } from "@/notification";

const router = useRouter();
const route = useRoute();

const id = route.query.token as string;

let closed = false;

const close = () => {
  if (closed) {
    return;
  }

  closed = true;

  router.go(-1);
};

const revoke = async () => {
  try {
    await revokeJoinToken(id);
  } catch (e) {
    showError("Failed to Revoke Token", e.message);
  }

  close();
}
</script>

<style scoped>
.heading {
  @apply flex gap-2 items-center mb-5 text-xl text-naturals-N14;
}
</style>
