<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="modal-window">
    <div class="heading">
      <h3 class="text-base text-naturals-N14 truncate flex-1">
        Delete the {{ object }} {{ id }} ?
      </h3>
      <close-button @click="close" />
    </div>
    <p class="text-xs">Please confirm the action.</p>

    <div class="flex justify-end gap-4 mt-8">
      <t-button @click="destroy" class="w-32 h-9">
        Delete
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
import { ManagementService } from "@/api/omni/management/management.pb";

const router = useRouter();
const route = useRoute();

const object = route.query.serviceAccount ? "Service Account" : "User";
const id = route.query.identity ?? route.query.serviceAccount;

let closed = false;

const close = () => {
  if (closed) {
    return;
  }

  closed = true;

  router.go(-1);
};

const destroy = async () => {
  let destroyed = true;

  if (route.query.serviceAccount) {
    const parts = (id as string).split("@");
    let name = parts[0];

    if (parts[1].indexOf("infra-provider") !== -1) {
      name = `infra-provider:${name}`;
    }

    try {
      await ManagementService.DestroyServiceAccount({
        name,
      });
    } catch (e) {
      showError("Failed to Delete the Service Account", e.message)

      return;
    }

    close();

    showSuccess(`Deleted Service Account ${id}`)

    return;
  }

  if (route.query.user) {
    try {
      await ResourceService.Delete({
        namespace: DefaultNamespace,
        type: UserType,
        id: route.query.user as string,
      }, withRuntime(Runtime.Omni));
    } catch (e) {
      if (e.code !== Code.NOT_FOUND) {
        showError("Failed to Remove the User", e.message)

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
        showError("Failed to Remove the Identity", e.message)

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
