<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="flex items-center justify-center">
    <div class="bg-naturals-N3 drop-shadow-md px-8 py-8 rounded-md flex flex-col gap-2">
      <div class="flex gap-4 items-center">
        <t-icon icon="kubernetes" class="fill-color w-6 h-6"/>
        <div class="text-xl font-bold text-naturals-N13">
          <div>Authenticate Kubernetes Access</div>
        </div>
      </div>

      <div v-if="!authRequestId" class="mx-12">
        Public key ID parameter is missing...
      </div>
      <template v-else>
        <div class="w-full flex flex-col gap-4">
          <div>The Kubernetes access is going to be granted for the user:</div>
          <user-info user="user" class="user-info"/>
          <div class="w-full flex flex-col gap-3">
            <t-button class="w-full" type="highlighted" @click="confirmOIDCRequest">Grant Access</t-button>
          </div>
        </div>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { showError } from "@/notification";
import { useRoute } from "vue-router";
import { OIDCService } from "@/api/omni/oidc/oidc.pb";

import TButton from "@/components/common/Button/TButton.vue";
import TIcon from "@/components/common/Icon/TIcon.vue";
import UserInfo from "@/components/common/UserInfo/UserInfo.vue";

const route = useRoute();

let authRequestId = route.params.authRequestId as string

const confirmOIDCRequest = async () => {
  try {
    const response = await OIDCService.Authenticate({
      auth_request_id: authRequestId,
    });

    window.location.href = response.redirect_url!;
  } catch (e) {
    showError("Failed to confirm authenticate request", e.message);

    throw e;
  }
}
</script>

<style scoped>
.user-info {
  @apply px-6 py-2 rounded-md bg-naturals-N6;
}
</style>
