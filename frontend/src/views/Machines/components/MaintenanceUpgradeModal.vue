<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { RadioGroup, RadioGroupLabel, RadioGroupOption } from '@headlessui/vue'
import { compare } from 'semver'
import { computed, nextTick, onUnmounted, ref, useTemplateRef, watch, watchEffect } from 'vue'

import type { Error } from '@/api/common/common.pb'
import { Runtime } from '@/api/common/omni.pb'
import {
  MaintenanceLifecycleRequestOperation,
  type MaintenanceLifecycleResponse,
  ManagementService,
} from '@/api/omni/management/management.pb'
import type { MachineStatusSpec, TalosVersionSpec } from '@/api/omni/specs/omni.pb'
import { withAbortController } from '@/api/options'
import { DefaultNamespace, MachineStatusType, TalosVersionType } from '@/api/resources'
import TCheckbox from '@/components/Checkbox/TCheckbox.vue'
import Modal from '@/components/Modals/Modal.vue'
import TAlert from '@/components/TAlert.vue'
import { majorMinorVersion } from '@/methods'
import { useResourceWatch } from '@/methods/useResourceWatch'

const { machineId } = defineProps<{
  machineId: string
}>()

const open = defineModel<boolean>('open', { default: false })

const selectedVersion = ref('')
const progress = ref<
  {
    level?: 'info' | 'error'
    message: string
  }[]
>([])

const upgrading = ref(false)
const finished = ref(false)
const inProgressFromServer = ref(false)

const { data: talosVersions, loading: talosVersionsLoading } = useResourceWatch<TalosVersionSpec>(
  () => ({
    skip: !open.value,
    resource: {
      type: TalosVersionType,
      namespace: DefaultNamespace,
    },
    runtime: Runtime.Omni,
  }),
)

const { data: machine, loading: machineLoading } = useResourceWatch<MachineStatusSpec>(() => ({
  skip: !open.value,
  resource: {
    type: MachineStatusType,
    namespace: DefaultNamespace,
    id: machineId,
  },
  runtime: Runtime.Omni,
}))

const versionMap = computed(() => new Map(talosVersions.value.map((v) => [v.metadata.id!, v])))
const currentVersion = computed(() => machine.value?.spec.talos_version?.slice(1))

// Seed once the source version is known so the radio comes up with a visible default; the disabled
// Upgrade button still prevents same-version submits.
watchEffect(() => {
  selectedVersion.value ||= currentVersion.value ?? ''
})

watch(open, (open) => {
  if (!open) return

  selectedVersion.value = ''
  progress.value = []
  upgrading.value = false
  finished.value = false
  inProgressFromServer.value = false
})

interface VersionGroup {
  versions: string[]
  unsupported: boolean
}

const upgradeVersions = computed(() => {
  if (!currentVersion.value) return {}

  const upgradeTargets =
    versionMap.value.get(currentVersion.value)?.spec.upgradable_talos_versions ?? []

  return [currentVersion.value, ...upgradeTargets]
    .map((v) => versionMap.value.get(v))
    .filter(
      (v): v is NonNullable<typeof v> =>
        !!v && (!v.spec.deprecated || v.spec.version === currentVersion.value),
    )
    .sort((a, b) => compare(b.spec.version!, a.spec.version!))
    .reduce<Record<string, VersionGroup>>((prev, { spec: { unsupported, version } }) => {
      const majorMinor = majorMinorVersion(version!)

      prev[majorMinor] ||= {
        versions: [],
        unsupported: true,
      }

      if (!unsupported || version === currentVersion.value) {
        prev[majorMinor].versions.push(version!)
        prev[majorMinor].unsupported = false
      }

      return prev
    }, {})
})

const versionsLoading = computed(() => talosVersionsLoading.value || machineLoading.value)

// once Upgrade fires, hide the form and show the streaming progress; keep the progress visible after
// the stream ends so the user sees the final outcome before dismissing.
const showProgress = computed(() => upgrading.value || finished.value)

const progressEl = useTemplateRef<HTMLElement>('progressEl')

watch(
  progress,
  async () => {
    const el = progressEl.value
    if (!el) {
      return
    }

    const wasPinned = el.scrollHeight - el.scrollTop - el.clientHeight < 24

    await nextTick()

    if (wasPinned) {
      el.scrollTop = el.scrollHeight
    }
  },
  { deep: true },
)

let abortController: AbortController

onUnmounted(() => abortController?.abort())
watchEffect(() => !open.value && abortController?.abort())

const doUpgrade = async () => {
  if (!selectedVersion.value || selectedVersion.value === currentVersion.value) {
    return
  }

  upgrading.value = true
  inProgressFromServer.value = false
  progress.value = []

  abortController = new AbortController()

  try {
    await ManagementService.MaintenanceLifecycle(
      {
        machine_id: machineId,
        operation: MaintenanceLifecycleRequestOperation.OPERATION_UPGRADE,
        version: selectedVersion.value,
      },
      (e: MaintenanceLifecycleResponse & { error?: Error }) => {
        if (e?.message) {
          progress.value.push({ level: 'info', message: e.message })
        } else if (e?.error) {
          throw e.error
        }
      },
      withAbortController(abortController),
    )

    finished.value = true
  } catch (e) {
    if (abortController.signal.aborted) return

    // The server's per-machine in-flight guard returns FailedPrecondition with this exact phrase when
    // a concurrent MaintenanceLifecycle is already running for the machine. Render it as a sticky
    // banner so the user has a clear, persistent explanation; other failures are logged.
    if (typeof e?.message === 'string' && e.message.includes('already in progress')) {
      inProgressFromServer.value = true
    } else {
      progress.value.push({ level: 'error', message: `[error] ${e?.message}` })
    }

    if (progress.value.length > 0) finished.value = true
  } finally {
    upgrading.value = false
  }
}
</script>

<template>
  <Modal
    v-model:open="open"
    title="Upgrade Talos"
    :action-label="finished ? undefined : 'Upgrade'"
    :cancel-label="finished ? 'Close' : 'Cancel'"
    :action-disabled="
      machineLoading ||
      !selectedVersion ||
      selectedVersion === currentVersion ||
      inProgressFromServer
    "
    :loading="upgrading"
    content-class="flex max-w-xl flex-col gap-2"
    @confirm="doUpgrade"
  >
    <template #description>Node {{ machineId }}</template>

    <template v-if="!versionsLoading && machine">
      <TAlert v-if="inProgressFromServer" type="warn" title="Upgrade in progress">
        A maintenance lifecycle operation is already in progress on this machine. Wait for it to
        finish before starting a new one.
      </TAlert>

      <div v-if="!showProgress" class="flex flex-col gap-2">
        <span v-if="!Object.keys(upgradeVersions).length">No versions found</span>

        <span class="text-sm">
          Pick a Talos version different from the one currently running to upgrade.
        </span>

        <RadioGroup
          id="talos-upgrade-version"
          v-model="selectedVersion"
          class="flex flex-1 flex-col gap-2 overflow-y-auto text-naturals-n13"
        >
          <template v-for="(group, label) in upgradeVersions" :key="label">
            <RadioGroupLabel
              as="div"
              class="sticky top-0 w-full bg-naturals-n4 p-1 pl-7 text-sm font-bold"
            >
              {{ `${label}${group.unsupported ? ' - Not supported by this Omni release' : ''}` }}
            </RadioGroupLabel>

            <div class="flex flex-col gap-1">
              <RadioGroupOption
                v-for="version in group.versions"
                :key="version"
                v-slot="{ checked }"
                :value="version"
              >
                <div
                  class="flex transform cursor-pointer items-center gap-2 px-2 py-1 text-sm transition-colors hover:bg-naturals-n4"
                  :class="{ 'bg-naturals-n4': checked }"
                >
                  <TCheckbox
                    :model-value="checked"
                    class="pointer-events-none"
                    @vue:mounted="
                      ($event) =>
                        checked && ($event.el as HTMLElement).scrollIntoView({ block: 'center' })
                    "
                  />
                  {{ version }}
                  <span v-if="version === currentVersion">(current)</span>
                </div>
              </RadioGroupOption>
            </div>
          </template>
        </RadioGroup>
      </div>

      <div v-else class="flex flex-col gap-2 overflow-hidden">
        <span class="shrink-0 text-sm">
          The upgrade runs on the server and continues even if you close this window.
        </span>

        <pre
          ref="progressEl"
          class="grow basis-80 overflow-y-auto rounded bg-naturals-n2 p-2 text-xs wrap-anywhere whitespace-pre-wrap text-naturals-n11"
        ><template v-if="progress.length > 0"><template v-for="line in progress" :key="line.message"><span v-if="line.level === 'info'" class="text-naturals-n11">{{ line.message }}
</span><span v-else-if="line.level === 'error'" class="text-red-r1">{{ line.message }}
</span></template></template><template v-else>Starting upgrade...</template>
        </pre>
      </div>
    </template>
  </Modal>
</template>
