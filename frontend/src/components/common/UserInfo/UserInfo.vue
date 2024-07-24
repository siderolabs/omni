<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="flex gap-2 items-center" :class="fontSize">
    <img v-if="avatar" class="rounded-full" :class="imageSize" :src="avatar as string" referrerpolicy="no-referrer"/>
    <div class="flex flex-col flex-1 truncate">
      <div class="text-naturals-N13 truncate">{{ fullname }}</div>
      {{ identity }}
    </div>
    <t-actions-box v-if="withLogoutControls" placement="top">
      <div @click="doLogout">
        <div class="px-4 py-2 cursor-pointer hover:text-naturals-N12">Log Out</div>
      </div>
    </t-actions-box>
  </div>
</template>

<script setup lang="ts">
import { useAuth0 } from '@auth0/auth0-vue';
import { computed } from 'vue';
import { resetKeys } from "@/methods/key";
import { currentUser } from "@/methods/auth";

import TActionsBox from "@/components/common/ActionsBox/TActionsBox.vue";
import { useRoute } from 'vue-router';
import { AuthType, authType } from '@/methods';

type Props = {
  withLogoutControls?: boolean;
  size?: "normal" | "small";
}

const props = withDefaults(defineProps<Props>(), {
  withLogoutControls: false,
  size: "normal",
});

const auth0 = useAuth0();

const route = useRoute();

const identity = computed(() => route.query.identity || auth0?.user?.value?.email?.toLowerCase() || window.localStorage.getItem("identity"));
const avatar = computed(() =>  route.query.avatar || auth0?.user?.value?.picture || window.localStorage.getItem("avatar"));
const fullname = computed(() => route.query.fullname || auth0?.user?.value?.name || window.localStorage.getItem("fullname"));

const fontSize = computed(() => {
  if (props.size === "small") {
    return { "text-xs": true };
  }

  return "";
});

const imageSize = computed(() => {
  if (props.size === "small") {
    return { "w-8": true, "h-8": true };
  }

  return { "w-12": true, "h-12": true };
});

const doLogout = async () => {
  await auth0?.logout({ logoutParams: { returnTo: window.location.origin } });

  resetKeys();

  currentUser.value = undefined;

  if (authType.value === AuthType.SAML) {
    location.reload();
  }
};
</script>
