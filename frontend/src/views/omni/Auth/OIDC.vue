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
          <div v-if="authCode" class="w-full flex items-center justify-center gap-0.5 p-1 border-naturals-N4 border rounded-lg pl-2">
            <div class="mr-2 text-sm text-naturals-N14">Access Code</div>
            <div class="flex-1"/>
            <div class="bg-naturals-N6 text-naturals-N14 font-roboto font-bold px-2 py-0.5 rounded-l-md cursor-pointer" @click="copyCode">
              {{ copied ? 'Copied' : authCode }}
            </div>
            <div class="bg-naturals-N6 text-naturals-N14 px-2 py-1 rounded-r-md hover:bg-naturals-N8 transition-colors cursor-pointer" @click="copyCode">
              <t-icon icon="copy" class="h-5"/>
            </div>
          </div>
          <div v-else class="w-full flex flex-col gap-3 my-0.5">
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
import { copyText } from "vue3-clipboard";

import TButton from "@/components/common/Button/TButton.vue";
import TIcon from "@/components/common/Icon/TIcon.vue";
import UserInfo from "@/components/common/UserInfo/UserInfo.vue";
import { ref } from "vue";

const route = useRoute();

let authRequestId = route.params.authRequestId as string

const authCode = ref<string>();

const confirmOIDCRequest = async () => {
  try {
    const response = await OIDCService.Authenticate({
      auth_request_id: authRequestId,
    });

    if (response.redirect_url) {
      window.location.href = response.redirect_url!;

      return;
    }

    authCode.value = response.auth_code;
  } catch (e) {
    showError("Failed to confirm authenticate request", e.message);

    throw e;
  }
}

const copied = ref(false);

let timeout: NodeJS.Timeout;

const copyCode = () => {
  clearTimeout(timeout);

  copied.value = true;

  timeout = setTimeout(() => {
    copied.value = false;
  }, 1000)

  copyText(authCode.value, undefined, () => {})
}
</script>

<style scoped>
.user-info {
  @apply px-6 py-2 rounded-md bg-naturals-N6;
}
</style>
