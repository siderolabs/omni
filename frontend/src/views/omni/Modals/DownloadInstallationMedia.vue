<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="modal-window flex flex-col gap-4">
    <div class="heading">
      <h3 class="text-base text-naturals-N14">
        Download Installation Media
      </h3>
      <close-button @click="close" />
    </div>

    <div v-if="phase !== Phase.Idle" class="flex flex-col items-center">
      <div class="flex gap-2 items-center"><document-arrow-down-icon class="w-5 h-5"/>{{ installationMedia?.spec.name }}</div>
      <div class="flex gap-2 items-center"><t-spinner class="w-5 h-5"/>
        <span v-if="phase === Phase.Loading">{{ downloaded }}</span>
        <span v-else>Generating Image</span>
      </div>
    </div>
    <template v-else>
      <div class="flex gap-3 flex-wrap">
        <div v-if="talosVersions" class="flex flex-wrap gap-4">
          <t-select-list @checkedValue="setTalosVersion" title="Talos Version" :defaultValue="DefaultTalosVersion" :values="talosVersions"
            :searcheable="true" />
        </div>

        <div v-if="!!optionsWatch && defaultValue" class="flex flex-wrap gap-4">
          <t-select-list @checkedValue="setOption" title="Options" :defaultValue="defaultValue" :values="optionNames"
            :searcheable="true" />
        </div>
      </div>

      <div class="flex">
        <h3 class="text-sm text-naturals-N14 flex-1">
          Pre-Install Extensions
        </h3>
        <t-checkbox
          class="col-span-2"
          label="Show Descriptions"
          @click="showDescriptions = !showDescriptions"
          :checked="showDescriptions"/>
      </div>

      <div class="flex flex-col gap-2">
        <t-input icon="search" v-model="filterExtensions"/>

        <div class="grid grid-cols-4 bg-naturals-N4 uppercase text-xs text-naturals-N13 pl-2 py-2">
          <div class="col-span-2">Name</div>
          <div>Version</div>
          <div>Author</div>
        </div>

        <Watch
          class="max-h-48 overflow-y-auto overflow-x-hidden"
          v-if="selectedTalosVersion"
          :opts="{
            resource: {
              id: selectedTalosVersion,
              type: TalosExtensionsType,
              namespace: DefaultNamespace,
            },
            runtime: Runtime.Omni,
          }"
          displayAlways
          >
          <template #default="{ items }">
            <div v-if="items[0]?.spec.items" class="flex flex-col">
              <div v-for="extension in filteredExtensions(items[0].spec.items!)" :key="extension.name" class="cursor-pointer flex gap-2 hover:bg-naturals-N5 transition-colors p-2 border-b border-naturals-N6 items-center"
                  @click="installExtensions[extension.name!] = !installExtensions[extension.name!]">
                <t-checkbox
                    class="col-span-2"
                    :checked="installExtensions[extension.name!]"/>
                <div class="grid grid-cols-4 gap-1 flex-1 items-center">
                  <div class="text-xs text-naturals-N13 col-span-2">
                    <WordHighlighter
                        :query="filterExtensions"
                        :textToHighlight="extension.name!.slice('siderolabs/'.length)"
                        highlightClass="bg-naturals-N14"
                    />
                  </div>
                  <div class="text-xs text-naturals-N13">{{ extension.version }}</div>
                  <div class="text-xs text-naturals-N13">{{ extension.author }}</div>
                  <div class="col-span-4 text-xs" v-if="extension.description && showDescriptions">
                    {{ extension.description }}
                  </div>
                </div>
              </div>
            </div>
            <div v-else class="flex gap-1 items-center text-xs p-4 text-primary-P2">
              <t-icon class="w-3 h-3" icon="warning"/>No extensions available for this Talos version
            </div>
          </template>
        </Watch>
      </div>

      <h3 class="text-sm text-naturals-N14 flex-1">
        Machine User Labels
      </h3>

      <div class="flex items-center gap-2">
        <Labels v-model="labels"/>
      </div>

      <h3 class="text-sm text-naturals-N14 flex-1">
        Additional Kernel Arguments
      </h3>

      <t-input v-model="kernelArguments"/>

      <!-- TODO(image-factory): enable this after further testing and making sure that it works -->
      <template v-if="false">
        <h3 class="text-sm text-naturals-N14 flex-1">
          Secure Boot
        </h3>

        <t-checkbox
            label="Enabled"
            :disabled="installationMedia?.spec?.no_secure_boot"
            @click="secureBoot = !secureBoot"
            :checked="secureBoot && !installationMedia?.spec?.no_secure_boot"/>
      </template>

      <h3 class="text-sm text-naturals-N14 flex-1">
        PXE Boot URL
      </h3>

      <div class="cursor-pointer px-1.5 py-1.5 rounded border border-naturals-N8 text-xs flex gap-2 items-center">
        <icon-button class="min-w-max" icon="refresh" @click="createSchematic" :icon-classes="{'animate-spin': creatingSchematic}"/>
        <span v-if="copiedPXEURL" class="flex-1 text-sm">Copied!</span>
        <span v-else class="flex-1 break-all" @click="createSchematic">{{ pxeURL ? pxeURL : 'Click to generate' }}</span>
        <icon-button class="min-w-max" icon="copy" @click="copyPXEURL"/>
      </div>

      <div>
        <p class="text-xs">The generated image will include the kernel arguments required to register with Omni automatically.</p>
      </div>

      <div class="flex justify-end gap-4">
        <t-button @click="close" class="w-32 h-9">
          Cancel
        </t-button>
        <t-button @click="download" class="w-32 h-9" type="highlighted">
          Download
        </t-button>
      </div>
    </template>
   </div>
</template>

<script setup lang="ts">
import { DefaultNamespace, DefaultTalosVersion, EphemeralNamespace, LabelsMeta, TalosExtensionsType, TalosVersionType, SecureBoot } from "@/api/resources";
import { useRouter } from "vue-router";
import { onUnmounted, ref, watch, computed, Ref } from "vue";
import { InstallationMediaType } from "@/api/resources";
import { InstallationMediaSpec, TalosExtensionsSpecInfo, TalosVersionSpec } from "@/api/omni/specs/omni.pb";
import { Runtime } from "@/api/common/omni.pb";
import { showError } from "@/notification";
import { formatBytes } from "@/methods";
import { Resource } from "@/api/grpc";

import CloseButton from "@/views/omni/Modals/CloseButton.vue";
import TSpinner from "@/components/common/Spinner/TSpinner.vue";
import TButton from "@/components/common/Button/TButton.vue";
import TSelectList from "@/components/common/SelectList/TSelectList.vue";
import Labels from "@/components/common/Labels/Labels.vue";
import WatchResource from "@/api/watch";
import Watch from "@/components/common/Watch/Watch.vue";
import TCheckbox from "@/components/common/Checkbox/TCheckbox.vue";
import TIcon from "@/components/common/Icon/TIcon.vue";
import TInput from "@/components/common/TInput/TInput.vue";
import WordHighlighter from "vue-word-highlighter";
import IconButton from "@/components/common/Button/IconButton.vue";

import { copyText } from "vue3-clipboard";
import { DocumentArrowDownIcon } from "@heroicons/vue/24/outline";
import { CreateSchematicRequest, ManagementService } from "@/api/omni/management/management.pb";

enum Phase {
  Idle = 0,
  Generating = 1,
  Loading = 2,
}

const installExtensions = ref<Record<string, boolean>>({})
const router = useRouter();
const phase = ref(Phase.Idle);
const showDescriptions = ref(false);
const fileSizeLoaded = ref(0);
const filterExtensions = ref("");
const kernelArguments = ref("");
const creatingSchematic = ref(false);

let controller: AbortController;
let closed = false;

const abortDownload = () => {
  phase.value = Phase.Idle;

  if (controller) {
    controller.abort();
  }
}

const close = () => {
  abortDownload();
  if (closed) {
    return;
  }

  closed = true;

  router.go(-1);
};

const labels: Ref<Record<string, {
  value: string
  canRemove: boolean
}>> = ref({});

const optionNames: Ref<string[]> = ref([]);

const watchOptions: Ref<Resource<InstallationMediaSpec>[]> = ref([]);

const options: Ref<Map<string, Resource<InstallationMediaSpec>>> = ref(new Map<string, Resource>());

const defaultValue = ref("")

const talosVersionsResources: Ref<Resource<TalosVersionSpec>[]> = ref([])

const optionsWatch = new WatchResource(watchOptions);
const talosVersionsWatch = new WatchResource(talosVersionsResources)
const schematicID = ref<string>();
const pxeBaseURL = ref<string>();
const secureBoot = ref(false);

const pxeURL = computed(() => {
  if (!pxeBaseURL.value || !installationMedia.value) {
    return;
  }

  let url = `${pxeBaseURL.value}/${selectedTalosVersion.value}/${installationMedia.value.spec.src_file_prefix}`;

  if (secureBoot.value && !installationMedia.value.spec.no_secure_boot) {
    url += "-secureboot";
  }

  return url;
});

optionsWatch.setup({
  runtime: Runtime.Omni,
  resource: {
    type: InstallationMediaType,
    namespace: EphemeralNamespace,
  },
});

talosVersionsWatch.setup({
  runtime: Runtime.Omni,
  resource: {
    type: TalosVersionType,
    namespace: DefaultNamespace,
  },
})

const talosVersions = computed(() => talosVersionsResources.value?.map(res => res.metadata.id!));

const selectedOption = ref("");
const selectedTalosVersion = ref(DefaultTalosVersion);
const filteredExtensions = (items: TalosExtensionsSpecInfo[]) => {
  if (!filterExtensions.value) {
    return items;
  }

  return items.filter(item => item.name?.includes(filterExtensions.value));
}

watch(() => optionsWatch.items?.value.length, () => {
  options.value = watchOptions.value.reduce((map, obj) => {
    return map.set(obj.spec.name!, obj);
  }, options.value);

  optionNames.value = watchOptions.value.map(item => item.spec.name!).sort()

  defaultValue.value = optionNames.value[0]
  selectedOption.value = defaultValue.value
});

watch([selectedOption, selectedTalosVersion, labels, installExtensions.value], () => {
  pxeBaseURL.value = undefined;
  schematicID.value = undefined;
});

onUnmounted(abortDownload);

const createSchematic = async () => {
  if (creatingSchematic.value) {
    return;
  }

  creatingSchematic.value = true;

  try {
    const schematic: CreateSchematicRequest = {
      extensions: [],
      extra_kernel_args: [],
      meta_values: {},
    };

    if (labels.value) {
      const l: Record<string, string> = {};
      for (const k in labels.value) {
        l[k] = labels.value[k].value;
      }

      schematic.meta_values![LabelsMeta] = JSON.stringify(l);
    }

    for (const key in installExtensions.value) {
      if (installExtensions.value[key]) {
        schematic.extensions?.push(key);
      }
    }

    schematic.extra_kernel_args = kernelArguments.value.split(" ").filter(item => item.trim());

    const resp = await ManagementService.CreateSchematic(schematic);

    pxeBaseURL.value = resp.pxe_url;
    schematicID.value = resp.schematic_id;

    return resp;
  } finally {
    creatingSchematic.value = false;
  }
}

const installationMedia = computed(() => {
  const downloadOption = options.value.get(selectedOption.value);
  if (!downloadOption) {
    return
  }

  return downloadOption;
})

const getFilename = (headers: Headers) => {
  const disposition = headers.get('Content-Disposition');
  if (!disposition) {
    throw new Error("no filename header in the response")
  }

  const parts = disposition.split(";");

  return parts[1].split("=")[1];
}

const download = async () => {
  abortDownload();

  controller = new AbortController();

  if (!installationMedia.value) {
    return;
  }

  const doRequest = async (url: string, init?: any) => {
    const resp = await fetch(url, init);

    if (!resp.ok) {
      throw new Error(`request failed: ${resp.status} ${await resp.text()}`);
    }

    return resp;
  }

  try {
    const schematicResponse = await createSchematic();

    let url = `/image/${schematicResponse.schematic_id}/v${selectedTalosVersion.value}/${installationMedia.value.metadata.id}`;

    if (secureBoot.value && !installationMedia.value.spec.no_secure_boot) {
      url += `?${SecureBoot}=true`;
    }

    phase.value = Phase.Generating;

    await doRequest(url, { signal: controller.signal, method: "HEAD" });

    phase.value = Phase.Loading;

    const resp = await doRequest(url, { signal: controller.signal });

    fileSizeLoaded.value = 0;

    const filename = getFilename(resp.headers);

    const res = new Response(new ReadableStream({
      async start(controller) {
        const reader = resp.body!.getReader();

        for (; ;) {
          const { done, value } = await reader.read();
          if (done) {
            break;
          }

          fileSizeLoaded.value += value.byteLength;
          controller.enqueue(value);
        }

        controller.close();
      },
    }));

    var a = document.createElement('a');
    const objectURL = window.URL.createObjectURL(await res.blob());
    a.style.display = "none";
    a.href = objectURL;
    a.download = filename;
    document.body.appendChild(a);
    a.click();
    window.URL.revokeObjectURL(objectURL);
    a.remove();
  } catch (e) {
    showError("Download Failed", e.message);

    throw e;
  } finally {
    close();
    phase.value = Phase.Idle;
  }
}

const setOption = (value: string) => {
  selectedOption.value = value;
}

const setTalosVersion = (value: string) => {
  selectedTalosVersion.value = value;
}

const copiedPXEURL = ref(false);

const copyPXEURL = async () => {
  if (!pxeURL.value) {
    await createSchematic();
  }

  copyText(pxeURL.value, undefined, () => {});

  copiedPXEURL.value = true;
  setTimeout(() => copiedPXEURL.value = false, 2000)
}

const downloaded = computed(() => {
  return formatBytes(fileSizeLoaded.value);
});
</script>

<style scoped>
.modal-window {
  @apply w-1/2 h-auto p-8;
}

.heading {
  @apply flex justify-between items-center text-xl text-naturals-N14;
}
</style>
