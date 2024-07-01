<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="modal-window flex flex-col max-h-screen my-4 gap-2">
    <div class="heading">
      <h3 class="text-base text-naturals-N14">Download Support Bundle</h3>
      <close-button @click="close"/>
    </div>

    <div class="flex-1 flex flex-col">

      <div class="header list-grid">
        <div>Source</div>
        <div>Progress</div>
      </div>
      <template v-if="sortedSources.length > 0">
        <t-list-item v-for="(source) in sortedSources" :key="source">
          <div class="flex gap-2 px-3">
            <div class="list-grid text-naturals-N12 flex-1">
              <div>{{ source }}</div>
              <div class="flex">
                <div class="flex gap-2 bg-naturals-N3 text-naturals-N13 rounded px-2 py-1 items-center text-xs">
                  <template v-if="done || sourceToProgress[source].done">
                    <template v-if="sourceToProgress[source].errors.length == 0">
                      <t-icon icon="check-in-circle" class="text-green-G1 w-4 h-4"/>
                      <span>Completed {{ sourceToProgress[source].text }}</span>
                    </template>
                    <template v-else>
                      <t-icon icon="error" class="text-red-R1 w-4 h-4"/>
                      <span>Completed with errors {{ sourceToProgress[source].text }}: {{ sourceToProgress[source].errors.join("\n") }}</span>
                    </template>
                  </template>
                  <template v-else>
                    <t-icon icon="loading" class="text-yellow-Y1 w-4 h-4 animate-spin"/>
                    <span>Downloading {{ sourceToProgress[source].text }}</span>
                  </template>
                </div>
              </div>
            </div>
          </div>
        </t-list-item>
      </template>
      <template v-else>
        <t-list-item>
          <div class="flex gap-2 px-3">
            <div class="list-grid text-naturals-N12 flex-1">
              <template v-if="started">
                <span>Started</span>
                <t-icon icon="loading" class="text-yellow-Y1 w-4 h-4 animate-spin"/>
              </template>
              <template v-else>
                <div>Not Started</div>
                <div>-</div>
              </template>
            </div>
          </div>
        </t-list-item>
      </template>
    </div>

    <div class="flex justify-end gap-4">
      <t-button @click="download" class="w-32 h-9" :disabled="!canDownloadSupportBundle || started" type="highlighted">
        <t-spinner v-if="!status" class="w-5 h-5"/>
        <span v-else>Download</span>
      </t-button>
      <t-button @click="close" class="w-32 h-9">
        <t-spinner v-if="!status" class="w-5 h-5"/>
        <span v-else>{{ closeText }}</span>
      </t-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { Ref, ref, computed, watch } from "vue";
import { DefaultNamespace, TalosUpgradeStatusType } from "@/api/resources";
import { useRoute, useRouter } from "vue-router";
import { Runtime } from "@/api/common/omni.pb";

import CloseButton from "@/views/omni/Modals/CloseButton.vue";
import TButton from "@/components/common/Button/TButton.vue";
import TSpinner from "@/components/common/Spinner/TSpinner.vue";
import Watch from "@/api/watch";
import { TalosUpgradeStatusSpec } from "@/api/omni/specs/omni.pb";
import { Resource } from "@/api/grpc";
import { setupClusterPermissions } from "@/methods/auth";
import { ManagementService } from "@/api/omni/management/management.pb";
import { showError } from "@/notification";
import { b64Decode } from "@/api/fetch.pb";
import TListItem from "@/components/common/List/TListItem.vue";
import TIcon from "@/components/common/Icon/TIcon.vue";

const route = useRoute();
const router = useRouter();

const selectedVersion = ref("");

const clusterName = route.params.cluster as string;

const resource = {
  namespace: DefaultNamespace,
  type: TalosUpgradeStatusType,
  id: clusterName,
};

const status: Ref<Resource<TalosUpgradeStatusSpec> | undefined> = ref();

const upgradeStatusWatch = new Watch(status);

upgradeStatusWatch.setup({
  resource: resource,
  runtime: Runtime.Omni,
});

watch(status, () => {
  if (selectedVersion.value === "") {
    selectedVersion.value = status.value?.spec.last_upgrade_version || "";
  }
});

let closed = false;

const close = () => {
  if (closed) {
    return;
  }

  closed = true;

  router.go(-1);
};

interface Progress {
  state?: string;
  errors: string[];
  currentNum: number;
  text: string;
  done: boolean;
}

const sourceToProgress: Ref<Record<string, Progress>> = ref({});
const sortedSources = computed(() => Object.keys(sourceToProgress.value).sort());

const started = ref(false);
const done = ref(false);

const closeText = computed(() => {
  return done.value ? "Close" : "Cancel";
});

const download = async () => {
  try {
    started.value = true;
    sourceToProgress.value = {};

    await ManagementService.GetSupportBundle({
      cluster: clusterName,
    }, resp => {
      if (resp?.progress?.source) {
        let current = sourceToProgress.value[resp.progress.source];

        if (!current) {
          current = {
            errors: [],
            currentNum: 0,
            text: "",
            done: false,
          }

          sourceToProgress.value[resp.progress.source] = current;
        }

        current.state = resp.progress.state;
        current.currentNum += 1;
        current.text = `${current.currentNum}/${resp.progress.total ?? '?'}`;
        current.done = current.currentNum === resp.progress.total;

        if (resp.progress.error) {
          current.errors?.push(resp.progress.error);
        }
      }

      if (resp.bundle_data) {
        const data = resp.bundle_data as unknown as string; // bundle_data is actually not a Uint8Array, but a base64 string
        const rawData = b64Decode(data);
        const blob = new Blob([rawData], { type: "application/zip" });

        const url = window.URL.createObjectURL(blob);
        downloadURL(url, "support.zip");

        done.value = true;

        return;
      }
    })
  } catch (e) {
    showError("Download Failed", e.message);
  }
}

const downloadURL = (data: string, fileName: string) => {
  const a = document.createElement('a')
  a.href = data
  a.download = fileName
  document.body.appendChild(a)
  a.style.display = 'none'
  a.click()
  a.remove()
}

const { canDownloadSupportBundle } = setupClusterPermissions(computed(() => route.params.cluster as string))
</script>

<style scoped>
.heading {
  @apply flex justify-between items-center mb-5 text-xl text-naturals-N14;
}

optgroup {
  @apply text-naturals-N14 font-bold;
}

.list-grid {
  @apply grid grid-cols-3 items-center justify-center;
}

.header {
  @apply bg-naturals-N2 mb-1 py-2 px-6 text-xs;
}
</style>