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
import type { TalosUpgradeStatusSpec } from '@/api/omni/specs/omni.pb'
import { DefaultNamespace, TalosUpgradeStatusType } from '@/api/resources'
import Watch from '@/api/watch'
import TButton from '@/components/common/Button/TButton.vue'
import TCheckbox from '@/components/common/Checkbox/TCheckbox.vue'
import TSpinner from '@/components/common/Spinner/TSpinner.vue'
import { updateTalos } from '@/methods/cluster'
import ManagedByTemplatesWarning from '@/views/cluster/ManagedByTemplatesWarning.vue'
import CloseButton from '@/views/omni/Modals/CloseButton.vue'

const route = useRoute()
const router = useRouter()

const selectedVersion = ref('')

const clusterName = route.params.cluster as string

const resource = {
  namespace: DefaultNamespace,
  type: TalosUpgradeStatusType,
  id: clusterName,
}

const status: Ref<Resource<TalosUpgradeStatusSpec> | undefined> = ref()

const upgradeStatusWatch = new Watch(status)

upgradeStatusWatch.setup({
  resource: resource,
  runtime: Runtime.Omni,
})

watch(status, () => {
  if (selectedVersion.value === '') {
    selectedVersion.value = status.value?.spec.last_upgrade_version || ''
  }
})

const upgradeVersions = computed(() => {
  if (!status?.value?.spec?.upgrade_versions) {
    return []
  }

  const sorted = [
    ...status.value.spec.upgrade_versions,
    status.value.spec.last_upgrade_version ?? '',
  ].sort(semver.compare)

  const result = {}

  for (const version of sorted) {
    const major = semver.major(version)
    const minor = semver.minor(version)

    const majorMinor = `${major}.${minor}`

    if (!result[majorMinor]) {
      result[majorMinor] = []
    }

    result[majorMinor].push(version)
  }

  return result
})

const action = computed(() => {
  if (!status.value) {
    return 'Loading...'
  }

  switch (semver.compare(selectedVersion.value, status.value.spec.last_upgrade_version ?? '')) {
    case 1:
      return 'Upgrade'
    case -1:
      return 'Downgrade'
  }

  return 'Unchanged'
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
  updateTalos(clusterName, selectedVersion.value)

  close()
}
</script>

<template>
  <div class="modal-window my-4 flex max-h-screen flex-col gap-2">
    <div class="heading">
      <h3 class="text-base text-naturals-N14">Update Talos</h3>
      <CloseButton @click="close" />
    </div>
    <ManagedByTemplatesWarning warning-style="popup" />
    <template v-if="status">
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
                <span v-if="version === status.spec.last_upgrade_version">(current)</span>
              </div>
            </RadioGroupOption>
          </div>
        </template>
      </RadioGroup>
    </template>

    <p class="text-xs">
      Changing the Talos version can result in control plane downtime. During this change you will
      be able to cancel the upgrade.
    </p>
    <p class="text-xs">This operation starts immediately.</p>

    <div class="flex justify-end gap-4">
      <TButton
        class="h-9 w-32"
        :disabled="!status || selectedVersion === status?.spec?.last_upgrade_version"
        type="highlighted"
        @click="upgradeClick"
      >
        <TSpinner v-if="!status" class="h-5 w-5" />
        <span v-else>{{ action }}</span>
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
