<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="flex flex-col gap-4">
    <div class="flex gap-1 items-start">
      <page-header :title="`${edit ? 'Edit Machine Class' : 'Create Machine Class'}`" class="flex-1" :subtitle="edit ? 'name: '+ route.params.classname as string : ''"/>
    </div>
    <div class="flex-1 flex items-center justify-center" v-if="loading">
      <t-spinner class="w-6 h-6"/>
    </div>
    <t-alert title="Not Found" type="error" v-else-if="notFound">
      The <code>MachineClass</code> {{ route.params.classname }} does not exist
    </t-alert>
    <template v-else>
      <div class="flex flex-col gap-2">
        <t-input v-if="!edit" title="Machine Class Name" v-model="machineClassName"/>
        <div class="flex gap-2 text-xs items-center" v-if="infraProviders.length > 0"><span>Machine Class Type:</span> <t-button-group v-model="machineClassMode" :options="machineClassModeOptions"/></div>
        <template v-if="machineClassMode === MachineClassMode.Manual">
          <div class="text-naturals-N13">Conditions</div>
          <div class="flex flex-wrap items-center gap-2">
            <template v-for="_, i in conditions" :key="i">
              <div class="flex gap-0.5 condition">
                <div
                  @click="deleteCondition(i)"
                  class="rounded-l-md flex items-center bg-naturals-N3 px-2 transition-colors hover:bg-naturals-N7 hover:text-naturals-N14 cursor-pointer">
                  <t-icon icon="delete" class="w-4 h-4"/>
                </div>
                <span role="textbox" ref="conditionElements" style="min-width: 28px"
                  @focus="lastFocused = i"
                  spellcheck="false"
                  class="text-sm font-roboto text-naturals-N14 px-2 py-1 whitespace-pre rounded-r-md bg-naturals-N3"
                  contenteditable
                  @keyup="(event: KeyboardEvent) => updateContent(i, event)"
                  @keydown.enter.prevent="addCondition"
                  @keydown.backspace="(event: KeyboardEvent) => handleBackspace(event, i)">
                  {{ conditions[i] }}
                </span>
              </div>
              <div v-if="i != conditions.length - 1">OR</div>
            </template>
            <icon-button icon="plus" class="h-full" @click="addCondition"/>
          </div>
          <div class="text-xs flex flex-col gap-1">
            <p>Using <code>,</code> in a single condition will match them using <code>AND</code> operator.</p>
            <p>Separate conditions are matched using <code>OR</code>.</p>
            <p>Allowed binary operators are
              <code>&gt;</code>,
              <code>&gt;=</code>,
              <code>&lt;</code>,
              <code>&lt;=</code>,
              <code>=</code>,
              <code>==</code>,
              <code>!=</code>,
              <code>in</code>,
              <code>notin</code>.
            </p>
            <p>Excluding a label can be done by prepending <code>!</code> to the label key, example: <code>!omni.sidero.dev/available</code>.</p>
          </div>
        </template>
        <template v-else>
          <provider-config v-model:infraProvider="infraProvider"/>
        </template>
      </div>
      <div class="flex flex-col flex-1 gap-2 mb-6">
        <div v-if="machineClassMode === MachineClassMode.Manual">
          <div class="text-naturals-N13">Matches</div>
          <Watch :opts="watchOpts" spinner no-records-alert errors-alert>
            <template #default="{ items }">
              <machine-match-item v-for="item in items" :key="itemID(item)" :machine="item" @filter-labels="copyLabel"/>
            </template>
          </Watch>
        </div>
        <template v-else>
          <machine-template
            v-if="infraProvider"
            :infra-provider="infraProvider"
            v-model:kernel-arguments="kernelArguments"
            v-model:grpc-tunnel="grpcTunnelMode"
            :provider-config="providerConfigs[infraProvider] || {}"
            @update:provider-config="value => { providerConfigs[infraProvider!] = value }"
            v-model:initial-labels="initialLabels"
            :key="infraProvider"
            />
        </template>
      </div>
      <div class="sticky -bottom-6 -my-6 -mx-6 bg-naturals-N1 border-t border-naturals-N5 h-16 flex items-center gap-2 px-12 py-6 text-xs justify-end">
        <t-button type="highlighted" :disabled="!canSubmit" @click="submit">
          {{ edit ? 'Update Machine Class' : 'Create Machine Class' }}
        </t-button>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { Resource, ResourceService } from "@/api/grpc";
import { withRuntime } from "@/api/options";
import { Runtime } from "@/api/common/omni.pb";
import { GrpcTunnelMode, MachineClassSpec } from "@/api/omni/specs/omni.pb";
import { DefaultNamespace, MachineStatusType, MachineClassType, InfraProviderStatusType, InfraProviderNamespace, LabelsMeta, LabelNoManualAllocation } from "@/api/resources";
import ItemWatch, { itemID } from "@/api/watch";
import { computed, ref, nextTick, Ref, watch, ComputedRef } from "vue";
import { useRoute, useRouter } from "vue-router";
import { showError } from "@/notification";
import { InfraProviderStatusSpec } from "@/api/omni/specs/infra.pb";
import yaml from "js-yaml";
import WatchResource from "@/api/watch";

import TAlert from "@/components/TAlert.vue";
import TSpinner from "@/components/common/Spinner/TSpinner.vue";
import PageHeader from "@/components/common/PageHeader.vue";
import Watch from "@/components/common/Watch/Watch.vue";
import TIcon from "@/components/common/Icon/TIcon.vue";
import TInput from "@/components/common/TInput/TInput.vue";
import MachineMatchItem from "./MachineMatchItem.vue";
import IconButton from "@/components/common/Button/IconButton.vue";
import TButton from "@/components/common/Button/TButton.vue";
import TButtonGroup from "@/components/common/Button/TButtonGroup.vue";
import MachineTemplate from "./MachineTemplate.vue";
import ProviderConfig from "./ProviderConfig.vue";

enum MachineClassMode {
  Manual = "Manual",
  AutoProvision = "Auto Provision",
}

const conditions = ref([""]);
const machineClassName = ref("");
const machineClassMode = ref(MachineClassMode.Manual)

const machineClassModeOptions: {label: string, value: any, tooltip: string, disabled?: boolean}[] = [{
  label: MachineClassMode.Manual,
  value: MachineClassMode.Manual,
  tooltip: "Use machines from the existing pool by selecting them using labels",
},
{
  label: MachineClassMode.AutoProvision,
  value: MachineClassMode.AutoProvision,
  tooltip: "Automatically provision machines from an infra provider",
}];

const infraProviders = ref<Resource<InfraProviderStatusSpec>[]>([]);

const infraProvidersWatch = new WatchResource(infraProviders);

infraProvidersWatch.setup({
  resource: {
    type: InfraProviderStatusType,
    namespace: InfraProviderNamespace,
  },
  runtime: Runtime.Omni
});

const props = defineProps<{edit: boolean}>();
const router = useRouter();
const route = useRoute();
const lastFocused = ref(0);

let loading: Ref<boolean> | ComputedRef<boolean>;
let notFound: Ref<boolean> | ComputedRef<boolean>;

const infraProvider = ref<string>();
const kernelArguments = ref<string>("");
const initialLabels = ref<Record<string, any>>({});
const grpcTunnelMode = ref<GrpcTunnelMode>(GrpcTunnelMode.UNSET);

const providerConfigs: Record<string, Record<string, any>> = {};

if (!props.edit) {
  notFound = ref(false);
  loading = ref(false);
}

let resourceVersion: string | undefined;

type Caret = {
  pos: number
  done?: boolean
}

// get the cursor position from element start
const getCursorPosition = (parent: Node, node: Node | null, offset: number, stat: Caret) => {
  if (stat.done) return stat;

  let currentNode: Node | undefined;
  if (parent.childNodes.length == 0) {
    stat.pos += parent.textContent?.length ?? 0;

    return stat;
  }

  for (let i = 0; i < parent.childNodes.length && !stat.done; i++) {
    currentNode = parent.childNodes[i];

    if (currentNode === node) {
      stat.pos += offset;
      stat.done = true;

      return stat;
    }

    getCursorPosition(currentNode, node, offset, stat);
  }

  return stat;
}

// find the child node and relative position and set it on range
const setCursorPosition = (parent: Node, range: Range, stat: Caret) => {
  if (stat.done) return range;

  if (parent.childNodes.length == 0) {
    if ((parent.textContent?.length ?? 0) >= stat.pos) {
      range.setStart(parent, stat.pos);
      stat.done = true;
    } else {
      stat.pos = stat.pos - (parent.textContent?.length ?? 0);
    }

    return range;
  }

  for (let i = 0; i < parent.childNodes.length && !stat.done; i++) {
    const currentNode = parent.childNodes[i];

    setCursorPosition(currentNode, range, stat);
  }

  return range;
}

// contains FF workaround: editable spans are losing caret position after getting vue reactive updates
// it has to save current element caret index before applying the change
// then apply the change and return caret position back
const updateContent = (i: number, event: KeyboardEvent) => {
  if (conditions.value[i] == event.target!['textContent']) {
    return;
  }

  const sel = window.getSelection?.();
  let caret: Caret | undefined;

  if (sel) {
    const node = sel.focusNode;
    const offset = sel.focusOffset;

    caret = getCursorPosition(event.target as Element, node, offset, { pos: 0, done: false });
  }

  conditions.value[i] = event.target!['textContent']

  nextTick(() => {
    if (sel && caret) {
      sel.removeAllRanges();

      const range = setCursorPosition(event.target as Node, document.createRange(), {
        pos: caret.pos,
        done: false,
      });

      range.collapse(true);
      sel.addRange(range);
    }
  })
}

let labels: Record<string, string> | undefined;

if (props.edit) {
  const machineClass: Ref<Resource<MachineClassSpec> | undefined> = ref();
  const machineClassWatch = new ItemWatch(machineClass);
  const route = useRoute();

  loading = machineClassWatch.loading;

  notFound = computed(() => {
    return machineClass.value === undefined;
  });

  machineClassName.value = route.params.classname as string;
  watch(() => route.params.classname, () => {
    machineClassName.value = route.params.classname as string;
  });

  machineClassWatch.setup(computed(() => {
    return {
      resource: {
        id: route.params.classname as string,
        namespace: DefaultNamespace,
        type: MachineClassType,
      },
      runtime: Runtime.Omni,
    }
  }));

  watch(machineClass, () => {
    machineClassMode.value = machineClass.value?.spec?.auto_provision ? MachineClassMode.AutoProvision : MachineClassMode.Manual;
    infraProvider.value = machineClass.value?.spec?.auto_provision?.provider_id;
    resourceVersion = machineClass.value?.metadata.version;
    labels = machineClass.value?.metadata.labels;

    kernelArguments.value = machineClass.value?.spec.auto_provision?.kernel_args?.join(" ") ?? "";

    const labelsMeta = machineClass.value?.spec.auto_provision?.meta_values?.find(item => item.key === LabelsMeta)
    if (labelsMeta) {
      initialLabels.value = {};

      const l = (yaml.load(labelsMeta.value!) as {machineLabels: Record<string, string>}).machineLabels;

      for (const key in l) {
        initialLabels.value[key] = {
          value: l[key],
          canRemove: true,
        }
      }
    }

    if (machineClass.value?.spec.auto_provision?.provider_id && machineClass.value?.spec.auto_provision?.provider_data) {
      providerConfigs[machineClass.value.spec.auto_provision.provider_id] = yaml.load(machineClass.value?.spec.auto_provision?.provider_data) as Record<string, any>;
    }

    const matchLabels = machineClass.value?.spec?.match_labels;
    if (!matchLabels) {
      return;
    }

    conditions.value = matchLabels;
  });
}

const placeCaretAtEnd = (el: any) => {
  if (typeof window.getSelection != "undefined"
    && typeof document.createRange != "undefined") {
    const range = document.createRange();
    range.selectNodeContents(el);
    range.collapse(false);
    const sel = window.getSelection();
    sel?.removeAllRanges();
    sel?.addRange(range);
  } else if (typeof document.body['createTextRange'] != "undefined") {
    const textRange = document.body['createTextRange']();
    textRange.moveToElementText(el);
    textRange.collapse(false);
    textRange.select();
  }
}

const watchOpts = computed(() => {
  return {
    resource: {
      namespace: DefaultNamespace,
      type: MachineStatusType,
    },
    selectors: nonEmptyConditions.value.map(c => c + `,!${LabelNoManualAllocation}`),
    selectUsingOR: true,
    runtime: Runtime.Omni,
  };
});

const conditionElements: Ref<Node & { focus: () => void, textContent: string}[] | null> = ref(null);

const updateFocus = () => {
  nextTick(() => {
    const node = conditionElements.value?.[conditions.value.length - 1];
    if (!node) {
      return;
    }

    node?.focus();
    placeCaretAtEnd(node);
  });
};

const addCondition = () => {
  conditions.value.push("");
  updateFocus();
}

const deleteCondition = (i: number) => {
  if (conditions.value.length === 1) {
    conditions.value[0] = '';

    return;
  }

  conditions.value.splice(i, 1);
}

const handleBackspace = (event: KeyboardEvent, i: number) => {
  if (conditions.value[i] !== "" || conditions.value.length < 2) {
    return;
  }

  event.preventDefault();
  conditions.value.splice(i, 1);
  updateFocus();
}

const copyLabel = (label: {key: string, value: string}) => {
  const block = `${label.key}${label.value ? ' = ' + label.value : '' }`;

  if (lastFocused.value >= conditions.value.length) {
    lastFocused.value = conditions.value.length - 1;
  }

  if (conditions.value[lastFocused.value].trim() === '') {
    conditions.value[lastFocused.value] = block;

    return;
  }

  conditions.value[lastFocused.value] += ", " + block;
};

const nonEmptyConditions = computed(() => {
  return conditions.value.filter(value => value.trim());
});

const canSubmit = computed(() => {
  if (machineClassName.value === "") {
    return false;
  }

  switch (machineClassMode.value) {
  case MachineClassMode.Manual:
    return nonEmptyConditions.value.length !== 0;
  case MachineClassMode.AutoProvision:
    return infraProvider.value !== undefined;
  }

  return false;
});

const submit = async () => {
  const machineClass: Resource<MachineClassSpec> = {
    metadata: {
      id: machineClassName.value,
      namespace: DefaultNamespace,
      type: MachineClassType,
      version: resourceVersion,
      labels,
    },
    spec: {
      match_labels: nonEmptyConditions.value,
    }
  };

  if (machineClassMode.value === MachineClassMode.AutoProvision && infraProvider.value) {
    machineClass.spec.auto_provision = {
      provider_id: infraProvider.value,
      grpc_tunnel: grpcTunnelMode.value,
    }

    if (kernelArguments.value.length > 0) {
      machineClass.spec.auto_provision.kernel_args = kernelArguments.value.split(" ");
    }

    if (initialLabels.value.length > 0) {
      const l: Record<string, string> = {};
      for (const k in initialLabels.value) {
        l[k] = initialLabels.value[k].value;
      }

      machineClass.spec.auto_provision.meta_values = [
        {
          key: LabelsMeta,
          value: yaml.dump({
            machineLabels: l,
          })
        }
      ]
    }

    const providerConfig = providerConfigs[infraProvider.value];

    if (providerConfig) {
      machineClass.spec.auto_provision.provider_data = yaml.dump(providerConfig);
    }
  }

  try {
    if (props.edit) {
      await ResourceService.Update(machineClass, resourceVersion, withRuntime(Runtime.Omni));
    } else {
      await ResourceService.Create(machineClass, withRuntime(Runtime.Omni));
    }
  } catch (e) {
    showError("Failed to Create Machine Class", e.message);

    return;
  }

  router.push({
    name: "MachineClasses"
  });
};
</script>

<style scoped>
.condition {
  @apply border border-opacity-0 rounded-md border-transparent transition-colors;
}

.condition:focus-within {
  @apply border-naturals-N8;
}

code {
  @apply font-roboto rounded bg-naturals-N6 px-1 py-0.5 text-naturals-N13;
}

.machine-template > * {
  @apply px-4 py-2 flex items-center gap-2;
}

.machine-template > * > *:first-child {
  @apply flex-1 whitespace-nowrap;
}
</style>
