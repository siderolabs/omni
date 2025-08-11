<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { RadioGroup, RadioGroupLabel, RadioGroupOption } from '@headlessui/vue'
import * as semver from 'semver'
import type { Ref } from 'vue'
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import type { Resource } from '@/api/grpc'
import { ManagementService } from '@/api/omni/management/management.pb'
import type { MachineStatusSpec, TalosVersionSpec } from '@/api/omni/specs/omni.pb'
import { DefaultNamespace, MachineStatusType, TalosVersionType } from '@/api/resources'
import Watch from '@/api/watch'
import TButton from '@/components/common/Button/TButton.vue'
import TCheckbox from '@/components/common/Checkbox/TCheckbox.vue'
import TSpinner from '@/components/common/Spinner/TSpinner.vue'
import { showError, showSuccess } from '@/notification'
import ManagedByTemplatesWarning from '@/views/cluster/ManagedByTemplatesWarning.vue'
import CloseButton from '@/views/omni/Modals/CloseButton.vue'

const router = useRouter()
const route = useRoute()

const selectedVersion = ref('')

const versions: Ref<Resource<TalosVersionSpec>[]> = ref([])

const versionsWatch = new Watch(versions)

versionsWatch.setup({
  resource: {
    type: TalosVersionType,
    namespace: DefaultNamespace,
  },
  runtime: Runtime.Omni,
})

const machine = ref<Resource<MachineStatusSpec>>()

const machineWatch = new Watch(machine)

machineWatch.setup({
  resource: {
    type: MachineStatusType,
    namespace: DefaultNamespace,
    id: route.query.machine as string,
  },
  runtime: Runtime.Omni,
})

watch(machine, () => {
  selectedVersion.value = machine.value?.spec.talos_version?.slice(1) ?? ''
})

const loading = versionsWatch.loading
const updating = ref(false)

const upgradeVersions = computed(() => {
  const sorted = new Array<Resource<TalosVersionSpec>>(...versions.value)

  sorted.sort((a, b) => {
    return semver.compare(a.metadata.id ?? '', b.metadata.id ?? '')
  })

  const result: Record<string, string[]> = {}

  for (const version of sorted) {
    if (version.spec.deprecated) {
      continue
    }

    const major = semver.major(version.metadata.id ?? '')
    const minor = semver.minor(version.metadata.id ?? '')

    const majorMinor = `${major}.${minor}`

    if (!result[majorMinor]) {
      result[majorMinor] = []
    }

    result[majorMinor].push(version.metadata.id!)
  }

  return result
})

let closed = false

const close = () => {
  if (closed) {
    return
  }

  closed = true

  router.go(-1)
}

const upgradeClick = async () => {
  if (machine.value?.spec.talos_version === `v${selectedVersion.value}`) {
    return
  }

  updating.value = true

  try {
    await ManagementService.MaintenanceUpgrade({
      machine_id: route.query.machine as string,
      version: selectedVersion.value,
    })
  } catch (e) {
    showError('Failed to Do Maintenance Update', e.message)

    return
  } finally {
    updating.value = false

    close()
  }

  showSuccess(
    'The Machine Update Triggered',
    `Machine ${route.query.machine} is being updated to Talos version ${selectedVersion.value}`,
  )
}
</script>

<template>
  <div class="modal-window flex h-1/2 flex-col gap-2">
    <div class="heading">
      <h3 class="text-base text-naturals-N14">Update Talos on Node {{ $route.query.machine }}</h3>
      <CloseButton @click="close" />
    </div>
    <ManagedByTemplatesWarning warning-style="popup" />
    <template v-if="!loading && machine">
      <RadioGroup
        id="k8s-upgrade-version"
        v-model="selectedVersion"
        class="flex flex-1 flex-col gap-2 overflow-y-auto text-naturals-N13"
      >
        <template v-for="(versions, group) in upgradeVersions" :key="group">
          <RadioGroupLabel as="div" class="w-full bg-naturals-N4 p-1 pl-7 text-sm font-bold">{{
            group
          }}</RadioGroupLabel>
          <div class="flex flex-col gap-1">
            <RadioGroupOption
              v-for="version in versions"
              :key="version"
              v-slot="{ checked }"
              :value="version"
            >
              <div
                class="tranform transition-color flex cursor-pointer items-center gap-2 px-2 py-1 text-sm hover:bg-naturals-N4"
                :class="{ 'bg-naturals-N4': checked }"
              >
                <TCheckbox :checked="checked" />
                {{ version }}
                <span v-if="version === machine.spec.talos_version?.slice(1)">(current)</span>
              </div>
            </RadioGroupOption>
          </div>
        </template>
      </RadioGroup>
    </template>
    <div v-else class="flex-1" />

    <div class="flex justify-end gap-4">
      <TButton
        class="h-9 w-32"
        :disabled="!versions || updating"
        type="highlighted"
        @click="upgradeClick"
      >
        <TSpinner v-if="!versions || updating" class="h-5 w-5" />
        <span v-else>Update</span>
      </TButton>
    </div>
  </div>
</template>

<style scoped>
.heading {
  @apply mb-5 flex items-center justify-between text-xl text-naturals-N14;
}
optgroup {
  @apply font-bold text-naturals-N14;
}
</style>
