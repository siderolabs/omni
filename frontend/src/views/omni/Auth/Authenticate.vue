<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="flex items-center justify-center">
    <div class="bg-naturals-N3 drop-shadow-md px-8 py-8 rounded-md flex flex-col gap-2">
      <div class="flex gap-4 items-center">
        <t-icon icon="key" class="fill-color w-6 h-6"/>
        <div class="text-xl font-bold text-naturals-N13">
          <div v-if="$route.query[AuthFlowQueryParam] === Auth.CLI">Authenticate CLI Access</div>
          <div v-else-if="$route.query[AuthFlowQueryParam] === Auth.Frontend">Authenticate UI Access</div>
          <div v-else-if="$route.query[AuthFlowQueryParam] === Auth.WorkloadProxy">Authenticate Workload Proxy Access</div>
        </div>
      </div>

      <div v-if="!publicKeyId && $route.query[AuthFlowQueryParam] === Auth.CLI" class="mx-12">
        Public key ID parameter is missing...
      </div>
      <template v-else>
        <div v-if="!authenticated">
          Redirecting to the authentication provider...
        </div>
        <div v-else-if="!identity">
          Could not get user's email address from the authentication provider...
        </div>
        <template v-else>
          <div v-if="confirmed">
            Successfully logged in as {{identity}}, you can return to the application...
          </div>
          <div v-else class="w-full flex flex-col gap-4">
            <div>The keys are going to be issued for the user:</div>
            <user-info user="user" class="user-info"/>
            <div class="w-full flex flex-col gap-3">
              <t-button type="secondary" class="w-full"  @click="switchUser">Switch User</t-button>
              <t-button v-if="$route.query[AuthFlowQueryParam] === Auth.CLI" class="w-full" type="highlighted" @click="confirmPublicKey">Grant Access</t-button>
              <t-button v-else class="w-full" type="highlighted" @click="generatePublicKey">Log In</t-button>
            </div>
          </div>
        </template>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useAuth0 } from "@auth0/auth0-vue";
import { computed, onMounted, ref } from "vue";
import { AuthService } from "@/api/omni/auth/auth.pb";
import { showError } from "@/notification";
import { AuthType, authType } from "@/methods";
import { useRoute, useRouter } from "vue-router";
import { User } from '@auth0/auth0-spa-js';
import {
  authBearerHeaderPrefix,
  samlSessionHeader,
  authHeader,
  authPublicKeyIDQueryParam,
  CLIAuthFlow,
  AuthFlowQueryParam, WorkloadProxyAuthFlow, RedirectQueryParam
} from "@/api/resources";
import { FrontendAuthFlow } from "@/router";
import { createKeys, saveKeys } from "@/methods/key";

import TButton from "@/components/common/Button/TButton.vue";
import TIcon from "@/components/common/Icon/TIcon.vue";
import UserInfo from "@/components/common/UserInfo/UserInfo.vue";
import { withMetadata } from "@/api/options";
import { fetchOption } from "@/api/fetch.pb";
import { Code } from "@/api/google/rpc/code.pb";
import { Auth0VueClient } from "@auth0/auth0-vue/src/global";

const user = ref<User | undefined>(undefined);
let idToken = "";
let logout: (value: any) => void;

/** @description Redirect the user's top-most window to the given URL.
 *
 * This makes sure that the redirect works correctly when the call comes from inside an iframe.
 *
 * @param url The URL to redirect to.
 */
const redirectToURL = (url: string) => {
  if (window.top) {
    window.top.location.href = url;
  } else {
    window.location.href = url;
  }
}

let auth0: Auth0VueClient | undefined;

onMounted(() => {
  if (authType.value === AuthType.Auth0) {
    auth0 = useAuth0();

    user.value = auth0.user.value;
    idToken = auth0.idTokenClaims.value!.__raw;

    logout = async () => {
      if (auth0) {
        await auth0.logout({
          logoutParams: {
            returnTo: window.location.origin
          }
        });
      }
    }
  } else {
    const navigateToLogin = () => {
      const query: string[] = [];
      for (const key in route.query) {
        query.push(`${key}=${route.query[key]}`)
      }

      redirectToURL(`/login?${query.join("&")}`);
    }

    if (!identity.value) {
      navigateToLogin();
    }

    logout = () => navigateToLogin();
  }
});

const authenticated = computed(() => {
  return identity.value;
});

const identity = computed(() => {
  if (authType.value === AuthType.Auth0) {
    return user.value?.email;
  }

  if (authType.value === AuthType.SAML) {
    return route.query.identity as string;
  }

  return undefined;
});

const name = computed(() => {
  if (authType.value === AuthType.Auth0) {
    return user.value?.name;
  }

  if (authType.value === AuthType.SAML) {
    return (route.query.fullname ?? route.query.identity) as string;
  }

  return "";
});

const picture = computed(() => {
  return user?.value?.picture;
});

const route = useRoute();
const router = useRouter();

let publicKeyId = route.query[authPublicKeyIDQueryParam] as string;

const confirmed = ref(false);
const redirect: string = route.query[RedirectQueryParam] as string;

const generatePublicKey = async () => {
  if (!identity.value) {
    return;
  }

  const res = await createKeys(identity.value);

  publicKeyId = res.publicKeyId;

  try {
    await confirmPublicKey();
  } catch (e) {
    return;
  }

  try {
    await saveKeys({
      email: identity.value,
      fullname: name.value ?? "",
      picture: picture.value ?? "",
    }, res.privateKey, res.publicKey, publicKeyId);
  } catch (e) {
    showError("Failed to save the key", e.message);

    return;
  }

  if (!redirect) {
    return;
  }

  if (redirect.indexOf('http://') === 0 || redirect.indexOf('https://') === 0) {
    redirectToURL(redirect)

    return;
  }

  await router.replace({ path: redirect });
}

let renewIdToken = false;

const confirmPublicKey = async () => {
  try {
    if (renewIdToken && auth0) {
      renewIdToken = false;

      await auth0.checkSession({
        cacheMode: "off"
      })

      user.value = auth0.user.value;
      idToken = auth0.idTokenClaims.value!.__raw;
    }

    const options: fetchOption[] = [];

    if (authType.value === AuthType.Auth0) {
      options.push(withMetadata({[authHeader]: authBearerHeaderPrefix + idToken}));
    } else if (authType.value === AuthType.SAML) {
      if (!route.query.session) {
        throw new Error("no session");
      }

      options.push(withMetadata({[samlSessionHeader]: route.query.session as string}));
    }

    await AuthService.ConfirmPublicKey({
      public_key_id: publicKeyId,
    }, ...options);

    confirmed.value = true
  } catch (e) {
    showError("Failed to confirm public key", e.message);

    if (e?.code == Code.UNAUTHENTICATED && auth0) {
      renewIdToken = true;
    }

    throw e;
  }
}

const switchUser = () => {
  logout({ returnTo: window.location.origin });
};

const Auth = {
  CLI: CLIAuthFlow,
  Frontend: FrontendAuthFlow,
  WorkloadProxy: WorkloadProxyAuthFlow,
}
</script>

<style scoped>
.user-info {
  @apply px-6 py-2 rounded-md bg-naturals-N6;
}
</style>
