<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="modal-window">
    <div class="heading">
      <h3 class="text-base text-naturals-N14 truncate flex-1">
        Delete the token {{ id }} ?
      </h3>
      <close-button @click="close" />
    </div>

    <join-token-warnings :id="id" @ready="isReady = true" class="flex-1 mb-2"/>

    <p class="text-xs text-primary-P2">This action CANNOT be undone. This will permanently delete the Join Token.</p>

    <div class="flex justify-end gap-4 mt-8">
      <t-button @click="deleteToken" class="w-32 h-9" icon="delete" :disabled="!isReady">
        Delete
      </t-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useRoute, useRouter } from "vue-router";

import CloseButton from "@/views/omni/Modals/CloseButton.vue";
import TButton from "@/components/common/Button/TButton.vue";
import { showError } from "@/notification";
import { deleteJoinToken } from "@/methods/auth";
import { ref } from "vue";
import JoinTokenWarnings from "@/views/omni/Modals/components/JoinTokenWarnings.vue";

const router = useRouter();
const route = useRoute();

const id = route.query.token as string;

const isReady = ref(false);

let closed = false;

const close = () => {
  if (closed) {
    return;
  }

  closed = true;

  router.go(-1);
};

const deleteToken = async () => {
  try {
    await deleteJoinToken(id);
  } catch (e) {
    showError("Failed to Delete Token", e.message);
  }

  close();
}
</script>

<style scoped>
.heading {
  @apply flex gap-2 items-center mb-5 text-xl text-naturals-N14;
}
</style>
