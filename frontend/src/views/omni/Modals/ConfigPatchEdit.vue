<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { Tab, TabGroup, TabPanel, TabPanels } from '@headlessui/vue'
import type { Ref } from 'vue'
import { ref, toRefs } from 'vue'

import { Code } from '@/api/google/rpc/code.pb'
import { ManagementService } from '@/api/omni/management/management.pb'
import TButton from '@/components/common/Button/TButton.vue'
import CodeEditor from '@/components/common/CodeEditor/CodeEditor.vue'
import TabButton from '@/components/common/Tabs/TabButton.vue'
import TabList from '@/components/common/Tabs/TabList.vue'
import TabsHeader from '@/components/common/Tabs/TabsHeader.vue'
import TAlert from '@/components/TAlert.vue'
import { closeModal } from '@/modal'

interface Props {
  onSave: (config: string, id?: string) => void
  tabs: { id: string; config: string }[]
}

const configs: Record<string, Ref<string>> = {}
const props = defineProps<Props>()

const { tabs } = toRefs(props)

for (const tab of tabs.value) {
  configs[tab.id] = ref(tab.config)
}

const err = ref<null | { message: string; title: string }>(null)

const close = () => {
  closeModal()
}

const save = async (): Promise<any> => {
  const promises: Promise<any>[] = []

  for (const id in configs) {
    err.value = null

    if (!configs[id].value) continue

    promises.push(
      ManagementService.ValidateConfig({
        config: configs[id].value,
      }).catch((e: { code?: Code; message?: string }) => {
        if (e.code === Code.INVALID_ARGUMENT) {
          err.value = {
            title: 'The Config is Invalid',
            message: e.message!,
          }
        } else {
          err.value = {
            title: 'Failed to Validate the Config',
            message: e.message!,
          }
        }
        throw e
      }),
    )
  }

  for (const id in configs) {
    if (props.onSave) {
      props.onSave(configs[id].value, id)
    }
  }

  return Promise.all(promises)
}

const saveAndClose = async () => {
  await save()
  close()
}

const openDocs = () => {
  window.open('https://www.talos.dev/latest/reference/configuration/', '_blank')
}
</script>

<template>
  <div class="modal-window" style="min-height: 350px">
    <div class="my-7 flex items-center px-8">
      <div class="heading">Edit Config Patch</div>
      <div class="flex flex-1 justify-end">
        <TButton icon="question" type="subtle" icon-position="left" @click="openDocs"
          >Config Reference</TButton
        >
      </div>
    </div>
    <div v-if="err" class="relative px-8">
      <TAlert
        class="absolute top-16 right-8 left-8 z-50"
        type="error"
        :title="err.title"
        :dismiss="{ action: () => (err = null), name: 'Close' }"
        >{{ err.message }}</TAlert
      >
    </div>
    <TabGroup as="div" class="flex flex-1 flex-col overflow-hidden">
      <TabList>
        <TabsHeader class="px-6">
          <Tab v-for="tab in tabs" v-slot="{ selected }" :key="tab.id">
            <TabButton :selected="selected">
              {{ tab.id }}
            </TabButton>
          </Tab>
        </TabsHeader>
      </TabList>
      <TabPanels as="template">
        <TabPanel v-for="tab in tabs" :key="tab.id" as="template">
          <div class="font-sm flex-1 overflow-y-auto bg-naturals-n2">
            <CodeEditor
              :value="configs[tab.id].value"
              @update:value="(updated) => (configs[tab.id].value = updated)"
            />
          </div>
        </TabPanel>
      </TabPanels>
    </TabGroup>
    <div class="flex justify-between gap-4 rounded-b bg-naturals-n3 p-4">
      <TButton type="secondary" @click="close">Cancel</TButton>
      <TButton @click="saveAndClose">Save</TButton>
    </div>
  </div>
</template>

<style scoped>
@reference "../../../index.css";

.modal-window {
  @apply h-2/3 w-2/3 p-0;
}
.heading {
  @apply text-xl text-naturals-n14;
}
</style>
