<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { RadioGroup, RadioGroupLabel, RadioGroupOption } from '@headlessui/vue'
import * as semver from 'semver'
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import { ManagementService } from '@/api/omni/management/management.pb'
import type {
  ClusterSpec,
  KubernetesUpgradeStatusSpec,
  KubernetesVersionSpec,
  TalosVersionSpec,
} from '@/api/omni/specs/omni.pb'
import { withContext } from '@/api/options'
import {
  ClusterType,
  DefaultNamespace,
  KubernetesUpgradeStatusType,
  KubernetesVersionType,
  TalosVersionType,
} from '@/api/resources'
import TButton from '@/components/common/Button/TButton.vue'
import TCheckbox from '@/components/common/Checkbox/TCheckbox.vue'
import TSpinner from '@/components/common/Spinner/TSpinner.vue'
import { useWatch } from '@/components/common/Watch/useWatch'
import { getDocsLink } from '@/methods'
import { upgradeKubernetes } from '@/methods/cluster'
import ManagedByTemplatesWarning from '@/views/cluster/ManagedByTemplatesWarning.vue'
import CloseButton from '@/views/omni/Modals/CloseButton.vue'

const route = useRoute()
const router = useRouter()

const runningPrechecks = ref(false)
const preCheckError = ref('')
const selectedVersion = ref('')

const clusterName = route.params.cluster as string

const { data: cluster } = useWatch<ClusterSpec>({
  resource: {
    type: ClusterType,
    namespace: DefaultNamespace,
    id: clusterName,
  },
  runtime: Runtime.Omni,
})

const { data: status } = useWatch<KubernetesUpgradeStatusSpec>({
  resource: {
    namespace: DefaultNamespace,
    type: KubernetesUpgradeStatusType,
    id: clusterName,
  },
  runtime: Runtime.Omni,
})

const { data: allK8sVersions } = useWatch<KubernetesVersionSpec>({
  resource: {
    namespace: DefaultNamespace,
    type: KubernetesVersionType,
  },
  runtime: Runtime.Omni,
})

const { data: allTalosVersionsUnsorted } = useWatch<TalosVersionSpec>({
  resource: {
    type: TalosVersionType,
    namespace: DefaultNamespace,
  },
  runtime: Runtime.Omni,
})

watch(status, () => {
  if (selectedVersion.value === '') {
    selectedVersion.value = status.value?.spec.last_upgrade_version || ''
  }
})

const supportedK8sVersions = computed(() =>
  (status.value?.spec.upgrade_versions ?? [])
    .concat(status.value?.spec.last_upgrade_version ?? '')
    .filter((v) => !!v)
    .sort(semver.compare),
)

interface VersionGroup {
  upgradeable: boolean
  versions: {
    upgradeable: boolean
    version: string
  }[]
}

const groupedK8sVersions = computed(() => {
  const oldestSupportedVersion = supportedK8sVersions.value[0]

  return allK8sVersions.value
    .map((v) => semver.parse(v.spec.version, false, true))
    .sort(semver.compare)
    .filter((v) => semver.gte(v, oldestSupportedVersion))
    .reduce<Record<string, VersionGroup>>((result, parsed) => {
      const { major, minor } = parsed
      const version = parsed.format()

      const majorMinor = `${major}.${minor}`

      result[majorMinor] ||= {
        upgradeable: isVersionUpgradeable(`${major}.${minor}.0`),
        versions: [],
      }

      // Only show individual versions if part of an upgradeable group
      if (result[majorMinor].upgradeable) {
        result[majorMinor].versions.push({
          upgradeable: isVersionUpgradeable(version),
          version,
        })
      }

      return result
    }, {})
})

const allTalosVersions = computed(() =>
  allTalosVersionsUnsorted.value
    .slice()
    .sort((a, b) => semver.compare(a.spec.version!, b.spec.version!)),
)

function isVersionUpgradeable(version: string) {
  return supportedK8sVersions.value.includes(version)
}

const k8sSupportMatrixDocsLink = computed(() =>
  getDocsLink('talos', '/getting-started/support-matrix', {
    talosVersion: cluster.value?.spec.talos_version,
  }),
)

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
  runningPrechecks.value = true
  preCheckError.value = ''

  try {
    const response = await ManagementService.KubernetesUpgradePreChecks(
      {
        new_version: selectedVersion.value,
      },
      withContext({ cluster: clusterName }),
    )

    if (response.ok) {
      upgradeKubernetes(clusterName, selectedVersion.value)

      close()
    } else {
      preCheckError.value = response.reason!
    }
  } catch (e) {
    preCheckError.value = e.message || e.toString()
  }

  runningPrechecks.value = false
}
</script>

<template>
  <div class="modal-window my-4 flex max-h-screen flex-col gap-2">
    <div class="mb-5 flex items-center justify-between text-xl text-naturals-n14">
      <h3 class="text-base text-naturals-n14">Update Kubernetes</h3>
      <CloseButton @click="close" />
    </div>
    <ManagedByTemplatesWarning warning-style="popup" />
    <template v-if="status">
      <RadioGroup
        id="k8s-upgrade-version"
        v-model="selectedVersion"
        class="flex max-h-64 flex-1 flex-col gap-2 overflow-y-auto text-naturals-n13"
      >
        <template
          v-for="({ upgradeable: groupUpgradeable, versions }, group) in groupedK8sVersions"
          :key="group"
        >
          <RadioGroupLabel
            as="div"
            class="sticky top-0 w-full bg-naturals-n4 p-1 pl-7 text-sm font-bold"
          >
            {{ group }}
            {{
              groupUpgradeable
                ? ''
                : allTalosVersions
                      .find((v) => v.spec.version === cluster?.spec.talos_version)
                      ?.spec.compatible_kubernetes_versions?.some((v) => semver.satisfies(v, group))
                  ? " - Can't skip minor version upgrades"
                  : ` - Requires Talos version ${allTalosVersions.find((v) => v.spec.compatible_kubernetes_versions?.some((v) => semver.satisfies(v, group)))?.spec.version}`
            }}
          </RadioGroupLabel>
          <div class="flex flex-col gap-1">
            <RadioGroupOption
              v-for="{ upgradeable, version } in versions"
              :key="version"
              v-slot="{ checked }"
              :value="version"
              :disabled="!upgradeable"
            >
              <div
                class="tranform transition-color flex cursor-pointer items-center gap-2 px-2 py-1 text-sm hover:bg-naturals-n4"
                :class="{ 'bg-naturals-n4': checked }"
              >
                <TCheckbox
                  :model-value="checked"
                  class="pointer-events-none"
                  :disabled="!upgradeable"
                  @vue:mounted="
                    ($event) =>
                      checked && ($event.el as HTMLElement).scrollIntoView({ block: 'center' })
                  "
                />
                {{ version }}
                <span v-if="version === status.spec.last_upgrade_version">(current)</span>
              </div>
            </RadioGroupOption>
          </div>
        </template>
      </RadioGroup>
    </template>

    <p class="text-xs">
      Downgrading minor versions is not supported. You can not skip minor version upgrades. You can
      only upgrade to versions supported by your Talos version. See the
      <a class="inline-block underline hover:mix-blend-lighten" :href="k8sSupportMatrixDocsLink">
        support matrix
      </a>
      for which versions are supported by your Talos version.
    </p>

    <p class="text-xs">
      Changing the Kubernetes version can result in control plane downtime. During this change you
      will be able to cancel the upgrade.
    </p>
    <p class="text-xs">This operation starts immediately.</p>

    <div v-if="runningPrechecks" class="text-xs text-primary-p3">
      Running pre-checks to validate the upgrade...
    </div>

    <div v-if="preCheckError" class="font-mono text-xs whitespace-pre-line text-primary-p3">
      {{ preCheckError }}
    </div>

    <div class="flex justify-end gap-4">
      <TButton
        class="h-9 w-32"
        :disabled="
          !status || runningPrechecks || selectedVersion === status?.spec?.last_upgrade_version
        "
        type="highlighted"
        @click="upgradeClick"
      >
        <TSpinner v-if="runningPrechecks || !status" class="h-5 w-5" />
        <span v-else>{{ action }}</span>
      </TButton>
    </div>
  </div>
</template>
