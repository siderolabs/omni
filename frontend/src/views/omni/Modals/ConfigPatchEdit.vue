<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="modal-window" style="min-height: 350px">
    <div class="flex px-8 my-7 items-center">
      <div class="heading">Edit Config Patch</div>
      <div class="flex justify-end flex-1">
        <t-button @click="openDocs" icon="question" type="subtle" iconPosition="left">Config Reference</t-button>
      </div>
    </div>
    <div class="px-8 relative" v-if="err">
      <t-alert
        class="absolute z-50 left-8 right-8 top-16"
        type="error"
        :title="err.title"
        :dismiss="{action: () => err = null, name: 'Close'}"
        >{{ err.message }}</t-alert>
    </div>
    <TabGroup as="div" class="flex-1 flex flex-col overflow-hidden">
      <TabList>
        <tabs-header class="px-6">
          <Tab v-slot="{ selected }" v-for="tab in tabs" :key="tab.id">
            <TabButton :selected="selected">
              {{ tab.id }}
            </TabButton>
          </Tab>
        </tabs-header>
      </TabList>
      <TabPanels as="template">
        <TabPanel as="template" :key="tab.id" v-for="tab in tabs">
          <div class="flex-1 overflow-y-auto font-sm bg-naturals-N2">
            <CodeEditor
                :value="configs[tab.id].value"
                @update:value="(updated) => configs[tab.id].value = updated"
            />
          </div>
        </TabPanel>
      </TabPanels>
    </TabGroup>
    <div class="flex p-4 gap-4 bg-naturals-N3 rounded-b justify-between">
      <t-button type="secondary" @click="close">Cancel</t-button>
      <t-button @click="saveAndClose">Save</t-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { Ref, ref, toRefs } from "vue";
import { closeModal } from "@/modal";
import { ManagementService } from "@/api/omni/management/management.pb";
import { Code } from "@/api/google/rpc/code.pb";

import TButton from "@/components/common/Button/TButton.vue";
import TAlert from "@/components/TAlert.vue";
import TabList from "@/components/common/Tabs/TabList.vue";
import TabButton from "@/components/common/Tabs/TabButton.vue";
import TabsHeader from "@/components/common/Tabs/TabsHeader.vue";

import { TabGroup, TabPanel, TabPanels, Tab } from "@headlessui/vue";

import CodeEditor from "@/components/common/CodeEditor/CodeEditor.vue";

interface Props {
  onSave: (config: string, id?: string) => void,
  tabs: {id: string, config: string}[],
}

const configs: Record<string, Ref<string>> = {};
const props = defineProps<Props>();

const { tabs } = toRefs(props);

for (const tab of tabs.value) {
  configs[tab.id] = ref(tab.config);
}

const err = ref<null | {message: string, title: string}>(null);

const close = () => {
  closeModal();
};

const save = async (): Promise<any> => {
  const promises: Promise<any>[] = [];

  for (const id in configs) {
    err.value = null;

    if (!configs[id].value)
      continue;

    promises.push(ManagementService.ValidateConfig({
      config: configs[id].value
    }).catch((e: { code?: Code, message?: string }) => {
      if (e.code === Code.INVALID_ARGUMENT) {
        err.value = {
          title: "The Config is Invalid",
          message: e.message!,
        };
      } else {
        err.value = {
          title: "Failed to Validate the Config",
          message: e.message!,
        };
      }
      throw e;
    }));
  }

  for (const id in configs) {
    if (props.onSave) {
      props.onSave(configs[id].value, id);
    }
  }

  return Promise.all(promises);
};

const saveAndClose = async () => {
  await save();
  close();
}

const openDocs = () => {
  window.open("https://www.talos.dev/latest/reference/configuration/", "_blank");
};
</script>

<style scoped>
.modal-window {
  @apply p-0 w-2/3 h-2/3;
}
.heading {
  @apply text-xl text-naturals-N14;
}
</style>
