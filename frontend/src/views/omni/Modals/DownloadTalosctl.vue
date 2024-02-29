<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="modal-window">
    <div class="heading">
      <h3 class="text-base text-naturals-N14">
        Download Talosctl
      </h3>
      <close-button @click="close"/>
    </div>

    <div class="flex flex-wrap gap-4 mb-5">
      <div v-if="selectedOption" class="flex flex-wrap gap-4">
        <t-select-list @checkedValue="setVersion" title="version" :defaultValue="defaultVersion"
                       :values="Object.keys(talosctlRelease?.release_data?.available_versions)"
                       :searcheable="true"/>
        <t-select-list @checkedValue="setOption" title="talosctl" :defaultValue="defaultValue"
                       :values="versionBinaries"
                       :searcheable="true"/>
      </div>
      <div v-else>
        <t-spinner class="w-6 h-6"/>
      </div>
    </div>

    <div>
      <p class="text-xs"><code>talosctl</code> can be used to access cluster nodes using Talos machine API.</p>
      <p class="text-xs">More downloads links can be found <a target="_blank" rel="noopener noreferrer" class="download-link text-xs" href="https://github.com/siderolabs/talos/releases">here</a>.</p>
    </div>

    <div class="flex justify-end gap-4 mt-8">
      <t-button @click="close" class="w-32 h-9">
        <span>Cancel</span>
      </t-button>
      <t-button @click="download" class="w-32 h-9">
        <span>Download</span>
      </t-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useRouter } from "vue-router";
import TButton from "@/components/common/Button/TButton.vue";
import TSelectList from "@/components/common/SelectList/TSelectList.vue";
import CloseButton from "@/views/omni/Modals/CloseButton.vue";
import { computed, onMounted, Ref, ref } from "vue";
import { showError } from "@/notification";
import TSpinner from "@/components/common/Spinner/TSpinner.vue";

const router = useRouter();
let closed = false;

const close = () => {
  if (closed) {
    return;
  }

  closed = true;

  router.go(-1);
};

onMounted(async () => {
  // Its sad that you cannot execute async function in computed
  const url = "/talosctl/downloads";

  try {
    const res = await fetch(url);
    talosctlRelease.value = await res.json();
  } catch (e) {
    showError(e.message);
    return;
  }

  defaultVersion.value = talosctlRelease.value.release_data.default_version
  selectedVersion.value = defaultVersion.value;

  defaultValue.value = talosctlRelease.value.release_data.available_versions[defaultVersion.value].find((item) => item.url.endsWith("linux-amd64"))?.name as string;
  selectedOption.value = defaultValue.value;
});

const talosctlRelease: Ref<ResponseData> = ref({});
const defaultValue = ref("");
const selectedOption = ref("");
const setOption = (value: string) => selectedOption.value = value;
const defaultVersion = ref("");
const selectedVersion = ref("");
const setVersion = (value: string) => selectedVersion.value = value;

const download = async () => {
  close();

  const link = talosctlRelease?.value?.release_data?.available_versions[selectedVersion.value].find((item) => item.name == selectedOption.value);
  if (!link) {
    return;
  }

  const a = document.createElement('a');
  a.href = link.url;
  document.body.appendChild(a);
  a.click();
  a.remove();
}

const versionBinaries = computed(() => {
  const result = talosctlRelease
    ?.value
    ?.release_data
    ?.available_versions[selectedVersion.value]
    ?.map((item) => {
      return item.name as string
    });

  if (!result) {
    return [] as string[];
  }

  return result;
});

interface ResponseData {
  status: string,
  release_data: ReleaseData,
}

interface ReleaseData {
  default_version: string,
  available_versions: {[key: string]: Asset[]},
}

interface Asset {
  name: string,
  url: string,
}
</script>

<style scoped>
.modal-window {
  @apply w-1/3 h-auto p-8;
}

.heading {
  @apply flex justify-between items-center mb-5 text-xl text-naturals-N14;
}

.download-link {
  @apply underline;
}
</style>
