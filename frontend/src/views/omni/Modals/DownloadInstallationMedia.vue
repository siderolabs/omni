<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="modal-window flex flex-col gap-4 overflow-y-scroll" style="height: 90%">
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

      <extensions-picker v-model="installExtensions" :talos-version="selectedTalosVersion" class="flex-1" :show-descriptions="showDescriptions"/>

      <h3 class="text-sm text-naturals-N14">
        Machine User Labels
      </h3>

      <div class="flex items-center gap-2">
        <Labels v-model="labels"/>
      </div>

      <h3 class="text-sm text-naturals-N14">
        Additional Kernel Arguments
      </h3>

      <t-input v-model="kernelArguments"/>

      <t-checkbox
          label="Secure Boot"
          :disabled="installationMedia?.spec?.no_secure_boot"
          @click="secureBoot = !secureBoot"
          :checked="secureBoot && !installationMedia?.spec?.no_secure_boot"/>

      <tooltip>
        <template #description>
          <div class="flex flex-col gap-1 p-2">
            <p>Configure Talos to use a GRPC tunnel for Siderolink (WireGuard) connection to Omni.</p>
            <p v-if="useGrpcTunnelDefault">As it is enabled in Omni on instance-level, it cannot be disabled for the installation media.</p>
          </div>
        </template>
        <t-checkbox :disabled="useGrpcTunnelDefault" :checked="useGrpcTunnel" display-checked-status-when-disabled
            label="Use Siderolink GRPC Tunnel"  @click="useGrpcTunnel = !useGrpcTunnel"/>
      </tooltip>

      <h3 class="text-sm text-naturals-N14">
        PXE Boot URL
      </h3>

      <div class="cursor-pointer px-1.5 py-1.5 rounded border border-naturals-N8 text-xs flex gap-2 items-center" :class="{'pointer-events-none': !supported}">
        <icon-button class="min-w-min" icon="refresh" @click="createSchematic" :icon-classes="{'animate-spin': creatingSchematic}" :disabled="!ready"/>
        <span v-if="copiedPXEURL" class="flex-1 text-sm">Copied!</span>
        <span v-else class="flex-1 break-all" @click="createSchematic">{{ pxeURL ? pxeURL : 'Click to generate' }}</span>
        <icon-button class="min-w-min" icon="copy" @click="copyPXEURL"/>
      </div>

      <div>
        <p v-if="supported" class="text-xs">The generated image will include the kernel arguments required to register with Omni automatically.</p>
        <p v-else class="text-xs text-primary-P2">{{ selectedOption }} supports only Talos version >= {{ minTalosVersion }}.</p>
      </div>

      <div class="flex justify-end gap-4">
        <t-button @click="close" class="w-32 h-9">
          Cancel
        </t-button>
        <t-button @click="download" class="w-32 h-9" type="highlighted" :disabled="!ready || !supported">
          Download
        </t-button>
      </div>
    </template>
   </div>
</template>

<script setup lang="ts">
import { DefaultNamespace, DefaultTalosVersion, EphemeralNamespace, LabelsMeta, TalosVersionType, SecureBoot, ConnectionParamsType, ConfigID } from "@/api/resources";
import { useRouter } from "vue-router";
import { onUnmounted, ref, watch, computed, Ref, onMounted } from "vue";
import { InstallationMediaType } from "@/api/resources";
import { InstallationMediaSpec, TalosVersionSpec } from "@/api/omni/specs/omni.pb";
import { Runtime } from "@/api/common/omni.pb";
import { showError } from "@/notification";
import { formatBytes } from "@/methods";
import { Resource, ResourceService } from "@/api/grpc";

import CloseButton from "@/views/omni/Modals/CloseButton.vue";
import TSpinner from "@/components/common/Spinner/TSpinner.vue";
import TButton from "@/components/common/Button/TButton.vue";
import TSelectList from "@/components/common/SelectList/TSelectList.vue";
import Labels from "@/components/common/Labels/Labels.vue";
import WatchResource from "@/api/watch";
import TCheckbox from "@/components/common/Checkbox/TCheckbox.vue";
import TInput from "@/components/common/TInput/TInput.vue";
import IconButton from "@/components/common/Button/IconButton.vue";
import yaml from "js-yaml";
import ExtensionsPicker from "@/views/omni/Extensions/ExtensionsPicker.vue";

import { copyText } from "vue3-clipboard";
import { DocumentArrowDownIcon } from "@heroicons/vue/24/outline";
import { CreateSchematicRequest, CreateSchematicRequestSiderolinkGRPCTunnelMode, ManagementService } from "@/api/omni/management/management.pb";
import Tooltip from "@/components/common/Tooltip/Tooltip.vue";
import { withRuntime } from "@/api/options";
import { ConnectionParamsSpec } from "@/api/omni/specs/siderolink.pb";

import * as semver from "semver";

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
const pxeURL = ref<string>();
const secureBoot = ref(false);

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
const useGrpcTunnel = ref(false);
const useGrpcTunnelDefault = ref(false);
const ready = ref(false);

const minTalosVersion = computed(() => {
  const option = options.value.get(selectedOption.value);
  if (!option) {
    return null;
  }

  return option.spec.min_talos_version;
});

const supported = computed(() => {
  if (minTalosVersion.value === null) {
    return false;
  }

  if (!minTalosVersion.value) {
    return true;
  }

  const selectedVersion = semver.parse(selectedTalosVersion.value, { loose: true });

  selectedVersion.prerelease = [];

  if (semver.lt(selectedVersion.format(), minTalosVersion.value, { loose: true })) {
    return false;
  }

  return true;
})

watch(() => optionsWatch.items?.value.length, () => {
  options.value = watchOptions.value.reduce((map, obj) => {
    return map.set(obj.spec.name!, obj);
  }, options.value);

  optionNames.value = watchOptions.value.map(item => item.spec.name!).sort()

  defaultValue.value = optionNames.value[0]
  selectedOption.value = defaultValue.value
});

watch([selectedOption, selectedTalosVersion, labels, installExtensions.value], () => {
  pxeURL.value = undefined;
  schematicID.value = undefined;
});

onMounted(async () => {
  const connectionParams: Resource<ConnectionParamsSpec> = await ResourceService.Get({
    namespace: DefaultNamespace,
    type: ConnectionParamsType,
    id: ConfigID,
  }, withRuntime(Runtime.Omni));

  useGrpcTunnel.value = connectionParams.spec.use_grpc_tunnel || false;
  useGrpcTunnelDefault.value = connectionParams.spec.use_grpc_tunnel || false;
  ready.value = true;
})

onUnmounted(abortDownload);

const createSchematic = async () => {
  if (creatingSchematic.value) {
    return;
  }

  creatingSchematic.value = true;

  const grpcTunnelMode = useGrpcTunnel.value ? CreateSchematicRequestSiderolinkGRPCTunnelMode.ENABLED : CreateSchematicRequestSiderolinkGRPCTunnelMode.DISABLED;

  try {
    const schematic: CreateSchematicRequest = {
      extensions: [],
      extra_kernel_args: [],
      meta_values: {},
      media_id:  installationMedia.value?.metadata.id,
      talos_version: selectedTalosVersion.value,
      secure_boot: secureBoot.value,
      siderolink_grpc_tunnel_mode: grpcTunnelMode,
    };

    if (labels.value && Object.keys(labels.value).length > 0) {
      const l: Record<string, string> = {};
      for (const k in labels.value) {
        l[k] = labels.value[k].value;
      }

      schematic.meta_values![LabelsMeta] = yaml.dump({
        machineLabels: l,
      });
    }

    for (const key in installExtensions.value) {
      if (installExtensions.value[key]) {
        schematic.extensions?.push(key);
      }
    }

    schematic.extra_kernel_args = kernelArguments.value.split(" ").filter(item => item.trim());

    const resp = await ManagementService.CreateSchematic(schematic);

    pxeURL.value = resp.pxe_url;
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

    let url = `/image/${schematicResponse!.schematic_id}/v${selectedTalosVersion.value}/${installationMedia.value.metadata.id}`;

    if (secureBoot.value && !installationMedia.value.spec.no_secure_boot) {
      url += `?${SecureBoot}=true`;
    }

    phase.value = Phase.Generating;

    await doRequest(url, { signal: controller.signal, method: "HEAD", headers: new Headers({ "Cache-Control": "no-store" }) });

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
