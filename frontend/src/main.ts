// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

import "@stardazed/streams-polyfill";
import { createApp } from 'vue';
import '@/index.css';
import App from '@/App.vue'
import VueClipboard from 'vue3-clipboard';
import AppUnavailable from '@/AppUnavailable.vue'
import router from '@/router';

import { initState, ResourceService, ResourceTyped } from "@/api/grpc";
import { AuthConfigID, AuthConfigType, DefaultNamespace } from "@/api/resources";
import { Runtime } from "@/api/common/omni.pb";
import { AuthConfigSpec } from "@/api/omni/specs/auth.pb";
import { AuthType, authType, suspended } from "@/methods";
import { createAuth0 } from "@auth0/auth0-vue";
import yaml from "js-yaml";
import { withRuntime } from "./api/options";
import vClickOutside from "click-outside-vue3"

const setupApp = async () => {
  let authConfigSpec: AuthConfigSpec | undefined = undefined

  try {
    const authConfig: ResourceTyped<AuthConfigSpec> = await ResourceService.Get({
      namespace: DefaultNamespace,
      type: AuthConfigType,
      id: AuthConfigID,
    }, withRuntime(Runtime.Omni));

    authConfigSpec = authConfig?.spec as AuthConfigSpec | undefined;
  } catch (e) {
    console.error("failed to get auth parameters", e)
    createApp(AppUnavailable).mount('#app');
    return
  }

  suspended.value = authConfigSpec?.suspended ?? false;

  if (authConfigSpec?.saml?.enabled) {
    authType.value = AuthType.SAML;
  } else if (authConfigSpec?.auth0?.enabled) {
    authType.value = AuthType.Auth0;
  }

  let app = createApp(App)
    .use(router)
    .use(VueClipboard, {
      autoSetContainer: true,
      appendToBody: true,
    })

  if (authType.value === AuthType.Auth0) {
    app = app.use(createAuth0({
      domain: authConfigSpec!.auth0?.domain!,
      client_id: authConfigSpec!.auth0?.client_id!,
      redirect_uri: window.location.origin,
    }))
  }

  app.use(vClickOutside);
  app.mount('#app');
}

initState();

setupApp()

window["jsyaml"] = yaml;

// noinspection JSUnusedGlobalSymbols
window["MonacoEnvironment"] = {
  getWorker(moduleId, label: string) {
    switch (label) {
      case "editorWorkerService":
        return new Worker(new URL("monaco-editor/esm/vs/editor/editor.worker", import.meta.url))
      case "yaml":
        return new Worker(new URL("monaco-yaml/yaml.worker", import.meta.url));
      default:
        throw new Error(`Unknown label ${label}`);
    }
  },
};