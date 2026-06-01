<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { RadioGroup, RadioGroupLabel, RadioGroupOption } from '@headlessui/vue'
import { compare, gte, parse, satisfies } from 'semver'
import { computed, ref, watchEffect } from 'vue'

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
import TCheckbox from '@/components/Checkbox/TCheckbox.vue'
import ManagedByTemplatesWarning from '@/components/ManagedByTemplatesWarning.vue'
import Modal from '@/components/Modals/Modal.vue'
import { getDocsLink } from '@/methods'
import { upgradeKubernetes } from '@/methods/cluster'
import { useResourceWatch } from '@/methods/useResourceWatch'
import { showError } from '@/notification'

const { clusterName } = defineProps<{
  clusterName: string
}>()

const open = defineModel<boolean>('open', { default: false })

const runningPrechecks = ref(false)
const selectedVersion = ref('')

const { data: cluster } = useResourceWatch<ClusterSpec>(() => ({
  skip: !open.value,
  resource: {
    type: ClusterType,
    namespace: DefaultNamespace,
    id: clusterName,
  },
  runtime: Runtime.Omni,
}))

const { data: status, loading: statusLoading } = useResourceWatch<KubernetesUpgradeStatusSpec>(
  () => ({
    skip: !open.value,
    resource: {
      namespace: DefaultNamespace,
      type: KubernetesUpgradeStatusType,
      id: clusterName,
    },
    runtime: Runtime.Omni,
  }),
)

const { data: allK8sVersions } = useResourceWatch<KubernetesVersionSpec>(() => ({
  skip: !open.value,
  resource: {
    namespace: DefaultNamespace,
    type: KubernetesVersionType,
  },
  runtime: Runtime.Omni,
}))

const { data: allTalosVersionsUnsorted } = useResourceWatch<TalosVersionSpec>(() => ({
  skip: !open.value,
  resource: {
    type: TalosVersionType,
    namespace: DefaultNamespace,
  },
  runtime: Runtime.Omni,
}))

watchEffect(() => {
  if (open.value) selectedVersion.value = status.value?.spec.last_upgrade_version || ''
})

const supportedK8sVersions = computed(() =>
  (status.value?.spec.upgrade_versions ?? [])
    .concat(status.value?.spec.last_upgrade_version ?? '')
    .filter((v) => !!v)
    .sort(compare),
)

interface VersionGroup {
  clusterSupportsGroup: boolean
  minTalosVersion: string
  upgradeable: boolean
  versions: {
    upgradeable: boolean
    version: string
  }[]
}

const groupedK8sVersions = computed(() => {
  const oldestSupportedVersion = supportedK8sVersions.value[0]
  const clusterTalosVersion = allTalosVersions.value.find(
    (v) => v.spec.version === cluster.value?.spec.talos_version,
  )

  return allK8sVersions.value
    .map((v) => parse(v.spec.version, false, true))
    .sort((a, b) => compare(b, a))
    .filter((v) => gte(v, oldestSupportedVersion))
    .reduce<Record<string, VersionGroup>>((result, parsed) => {
      const { major, minor } = parsed
      const version = parsed.format()

      const majorMinor = `${major}.${minor}`

      result[majorMinor] ||= {
        clusterSupportsGroup:
          clusterTalosVersion?.spec.compatible_kubernetes_versions?.some((v) =>
            satisfies(v, majorMinor),
          ) ?? false,
        minTalosVersion:
          allTalosVersions.value.find((v) =>
            v.spec.compatible_kubernetes_versions?.some((v) => satisfies(v, majorMinor)),
          )?.spec.version ?? 'unknown',
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
  allTalosVersionsUnsorted.value.toSorted((a, b) => compare(b.spec.version!, a.spec.version!)),
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

  switch (compare(selectedVersion.value, status.value.spec.last_upgrade_version ?? '')) {
    case 1:
      return 'Upgrade'
    case -1:
      return 'Downgrade'
  }

  return 'Unchanged'
})

const upgradeClick = async () => {
  runningPrechecks.value = true

  try {
    const response = await ManagementService.KubernetesUpgradePreChecks(
      {
        new_version: selectedVersion.value,
      },
      withContext({ cluster: clusterName }),
    )

    if (!response.ok) {
      throw new Error(response.reason)
    }

    await upgradeKubernetes(clusterName, selectedVersion.value)

    open.value = false
  } catch (e) {
    showError(e instanceof Error ? e.message : String(e))
  } finally {
    runningPrechecks.value = false
  }
}
</script>

<template>
  <Modal
    v-model:open="open"
    title="Update Kubernetes"
    :action-label="action"
    :action-disabled="
      !status || runningPrechecks || selectedVersion === status?.spec?.last_upgrade_version
    "
    :loading="statusLoading || runningPrechecks"
    content-class="flex min-h-0 max-w-xl flex-1 flex-col gap-2"
    @confirm="upgradeClick"
  >
    <div class="shrink-0">
      <ManagedByTemplatesWarning warning-style="popup" />
    </div>

    <RadioGroup
      v-if="status"
      v-model="selectedVersion"
      class="flex max-h-64 min-h-16 flex-1 flex-col gap-2 overflow-y-auto text-naturals-n13"
    >
      <template
        v-for="(
          { minTalosVersion, clusterSupportsGroup, upgradeable: groupUpgradeable, versions }, group
        ) in groupedK8sVersions"
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
              : clusterSupportsGroup
                ? " - Can't skip minor version upgrades"
                : ` - Requires Talos version ${minTalosVersion}`
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

    <p class="shrink-0 text-xs">
      Downgrading minor versions is not supported. You can not skip minor version upgrades. You can
      only upgrade to versions supported by your Talos version. See the
      <a class="link-primary" :href="k8sSupportMatrixDocsLink">support matrix</a>
      for which versions are supported by your Talos version.
    </p>

    <p class="shrink-0 text-xs">
      Changing the Kubernetes version can result in control plane downtime. During this change you
      will be able to cancel the upgrade.
    </p>
    <p class="shrink-0 text-xs">This operation starts immediately.</p>

    <div v-if="runningPrechecks" class="shrink-0 text-xs text-primary-p3">
      Running pre-checks to validate the upgrade...
    </div>
  </Modal>
</template>
