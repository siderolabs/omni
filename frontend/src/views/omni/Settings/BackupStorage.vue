<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="flex flex-col">
    <div class="flex gap-1 items-start">
      <page-header title="Configure Backups Storage" class="flex-1"/>
    </div>
    <t-alert v-if="error" title="Failed to Fetch Current Storage State" type="error">
      {{ error }}
    </t-alert>
    <div class="flex flex-col gap-5" v-else-if="ready && !saving" @keydown.enter="updateConfig">
      <div class="text-naturals-N14 font-bold">
        Storage Type {{ `${store.charAt(0).toUpperCase()}${store.slice(1)}` }}
      </div>
      <template v-if="s3Spec">
        <t-input title="Endpoint" :model-value="s3Spec.endpoint || ''" @update:model-value="value => s3Spec.endpoint = value"/>
        <t-input title="Bucket" :model-value="s3Spec.bucket || ''" @update:model-value="value => s3Spec.bucket = value"/>
        <t-input title="Region" :model-value="s3Spec.region || ''" @update:model-value="value => s3Spec.region = value"/>
        <div class="flex gap-5">
          <t-input title="Access Key ID" type="password" class="flex-1 flex-shrinl" :model-value="s3Spec.access_key_id || ''" @update:model-value="value => s3Spec.access_key_id = value"/>
          <t-input title="Secret Access Key" type="password" class="flex-1" :model-value="s3Spec.secret_access_key || ''" @update:model-value="value => s3Spec.secret_access_key = value"/>
        </div>
        <t-input title="Session Token" :model-value="s3Spec.session_token || ''" @update:model-value="value => s3Spec.session_token = value"/>
        <div class="place-self-end flex gap-2">
          <t-button :disabled="!canManageBackupStore" @click="resetConfig">
            Reset
          </t-button>
          <t-button :disabled="!canManageBackupStore" @click="updateConfig" type="highlighted">
            Save
          </t-button>
        </div>
      </template>
    </div>
    <div class="flex flex-1 items-center justify-center" v-else>
      <t-spinner class="w-6 h-6"/>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from "vue";
import { canManageBackupStore } from "@/methods/auth";
import { Resource, ResourceService, ResourceTyped } from "@/api/grpc";
import { EtcdBackupOverallStatusSpec, EtcdBackupS3ConfSpec } from "@/api/omni/specs/omni.pb";
import { Runtime } from "@/api/common/omni.pb";
import { DefaultNamespace, EtcdBackupOverallStatusID, EtcdBackupOverallStatusType, EtcdBackupS3ConfID, EtcdBackupS3ConfType } from "@/api/resources";
import { withRuntime } from "@/api/options";
import { Code } from '@/api/google/rpc/code.pb';

import TButton from "@/components/common/Button/TButton.vue";
import PageHeader from "@/components/common/PageHeader.vue";
import TInput from "@/components/common/TInput/TInput.vue";
import TSpinner from "@/components/common/Spinner/TSpinner.vue";
import TAlert from "@/components/TAlert.vue";
import { showError, showSuccess } from "@/notification";

const ready = ref(false);
let s3Spec = ref<EtcdBackupS3ConfSpec>({});
const store = ref("unknown");
const error = ref<string>();
const saving = ref(false);

const s3ConfigMetadata = {
  id: EtcdBackupS3ConfID,
  type: EtcdBackupS3ConfType,
  namespace: DefaultNamespace,
};

onMounted(async () => {
  const status: Resource<EtcdBackupOverallStatusSpec> = await ResourceService.Get({
    id: EtcdBackupOverallStatusID,
    type: EtcdBackupOverallStatusType,
    namespace: DefaultNamespace,
  }, withRuntime(Runtime.Omni));

  store.value = status.spec.configuration_name!;

  if (status.spec.configuration_name !== "s3") {
    ready.value = true;

    return;
  }

  try {
    const config = await ResourceService.Get(s3ConfigMetadata, withRuntime(Runtime.Omni));

    s3Spec.value = config?.spec;
  } catch (e) {
    if (e.code !== Code.NOT_FOUND) {
      error.value = e.message;
    }

    s3Spec.value = {};
  } finally {
    ready.value = true;
  }
});

const resetConfig = async () => {
  saving.value = true

  try {
    await ResourceService.Delete(s3ConfigMetadata, withRuntime(Runtime.Omni));
  } catch (e) {
    if (e.code !== Code.NOT_FOUND) {
      showError("Failed to Update Storage Config", e.message);

      saving.value = false;

      return;
    }
  } finally {
    saving.value = false;
  }

  s3Spec.value = {};

  showSuccess("The Backup Storage Config was Reset")
};

const updateConfig = async () => {
  if (Object.keys(s3Spec.value).length === 0) {
    showError("Failed to Update Storage Config", "No configuration was specified");

    return;
  }

  saving.value = true
  let config: ResourceTyped<EtcdBackupS3ConfSpec> = {
    metadata: s3ConfigMetadata,
    spec: {},
  };

  let exists: boolean = false;

  try {
    config = await ResourceService.Get(s3ConfigMetadata, withRuntime(Runtime.Omni));

    exists = true;
  } catch (e) {
    if (e.code !== Code.NOT_FOUND) {
      showError("Failed to Update Storage Config", e.message);

      saving.value = false;

      return;
    }
  }

  config.spec = s3Spec.value;

  try {
    if (exists) {
      await ResourceService.Update(config, config.metadata.version || 0, withRuntime(Runtime.Omni));
    } else {
      await ResourceService.Create(config, withRuntime(Runtime.Omni));
    }
  } catch (e) {
    showError("Failed to Update Storage Config", e.message);

    return;
  } finally {
    saving.value = false
  }


  showSuccess("The Backup Storage Config was Updated")
};
</script>
