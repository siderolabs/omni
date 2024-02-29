<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="modal-window">
    <div class="heading">
      <h3 class="text-base text-naturals-N14 truncate flex-1">
        Destroy the User {{ $route.query.identity }} ?
      </h3>
      <close-button @click="close" />
    </div>
    <p class="text-xs">Please confirm the action.</p>

    <div class="flex justify-end gap-4 mt-8">
      <t-button @click="destroyResources" class="w-32 h-9">
        Destroy
      </t-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { DefaultNamespace, IdentityType, UserType } from "@/api/resources";
import { useRoute, useRouter } from "vue-router";
import { showError, showSuccess } from "@/notification";
import { ResourceService } from "@/api/grpc";
import { Code } from '@/api/google/rpc/code.pb';
import { Runtime } from "@/api/common/omni.pb";

import CloseButton from "@/views/omni/Modals/CloseButton.vue";
import TButton from "@/components/common/Button/TButton.vue";
import { withRuntime } from "@/api/options";

const router = useRouter();
const route = useRoute();

let closed = false;

const close = () => {
  if (closed) {
    return;
  }

  closed = true;

  router.go(-1);
};

const destroyResources = async () => {
  let destroyed = true;

  if (route.query.user) {
    try {
      await ResourceService.Delete({
        namespace: DefaultNamespace,
        type: UserType,
        id: route.query.user as string,
      }, withRuntime(Runtime.Omni));
    } catch (e) {
      if (e.code !== Code.NOT_FOUND) {
        showError("Failed to remove the user", e.message)

        destroyed = false;
      }
    }
  }

  if (route.query.identity) {
    try {
      await ResourceService.Delete({
        namespace: DefaultNamespace,
        type: IdentityType,
        id: route.query.identity as string,
      }, withRuntime(Runtime.Omni));
    } catch (e) {
      if (e.code !== Code.NOT_FOUND) {
        showError("Failed to remove the identity", e.message)

        destroyed = false;
      }
    }
  }

  close();

  if (destroyed)
    showSuccess(
      `The User ${route.query.identity} was Destroyed`
    );
}
</script>

<style scoped>
.heading {
  @apply flex gap-2 items-center mb-5 text-xl text-naturals-N14;
}
</style>
