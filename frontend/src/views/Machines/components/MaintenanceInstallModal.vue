<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { RadioGroup, RadioGroupLabel, RadioGroupOption } from '@headlessui/vue'
import { compare } from 'semver'
import { computed, nextTick, onUnmounted, ref, useTemplateRef, watch, watchEffect } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import {
  MaintenanceLifecycleRequestOperation,
  ManagementService,
} from '@/api/omni/management/management.pb'
import type { MachineStatusSpec, TalosVersionSpec } from '@/api/omni/specs/omni.pb'
import { withAbortController } from '@/api/options'
import { DefaultNamespace, MachineStatusType, TalosVersionType } from '@/api/resources'
import TCheckbox from '@/components/Checkbox/TCheckbox.vue'
import Modal from '@/components/Modals/Modal.vue'
import TSelectList from '@/components/SelectList/TSelectList.vue'
import TAlert from '@/components/TAlert.vue'
import { majorMinorVersion } from '@/methods'
import { useResourceWatch } from '@/methods/useResourceWatch'
import { showError } from '@/notification'

const { machineId } = defineProps<{
  machineId: string
}>()

const open = defineModel<boolean>('open', { default: false })

const selectedVersion = ref('')
const selectedDisk = ref('')
const progress = ref<string[]>([])
const installing = ref(false)
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

// Maintenance install writes the chosen version to disk; the running (in-memory) version is itself
// a valid — and usually the default — install target, so seed it as soon as it's known.
watchEffect(() => {
  selectedVersion.value ||= currentVersion.value ?? ''
})

watch(open, (open) => {
  if (!open) return

  selectedVersion.value = ''
  selectedDisk.value = ''
  progress.value = []
  installing.value = false
  finished.value = false
  inProgressFromServer.value = false
})

// installable disks: skip read-only devices, CD-ROMs, and any device missing a linux_name (the
// install target is the linux_name path — without one there's nothing to pass to Talos).
const disks = computed(
  () =>
    machine.value?.spec.hardware?.blockdevices?.flatMap((device) => {
      if (device.readonly || device.type === 'CD' || !device.linux_name) {
        return []
      }

      return [device.linux_name]
    }) ?? [],
)

watch(disks, (newDisks) => {
  if (!selectedDisk.value || !newDisks.includes(selectedDisk.value)) {
    selectedDisk.value = newDisks[0] ?? ''
  }
})

interface VersionGroup {
  versions: string[]
  unsupported: boolean
}

const installVersions = computed(() => {
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

// once Install fires, hide the form and show the streaming progress; keep the progress visible after
// the stream ends so the user sees the final outcome before dismissing.
const showProgress = computed(() => installing.value || finished.value)

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

const doInstall = async () => {
  if (!selectedVersion.value || !selectedDisk.value) {
    return
  }

  installing.value = true
  inProgressFromServer.value = false
  progress.value = []

  abortController = new AbortController()

  try {
    await ManagementService.MaintenanceLifecycle(
      {
        machine_id: machineId,
        operation: MaintenanceLifecycleRequestOperation.OPERATION_INSTALL,
        version: selectedVersion.value,
        disk: selectedDisk.value,
      },
      ({ message }) => {
        if (message) progress.value.push(message)
      },
      withAbortController(abortController),
    )

    finished.value = true
  } catch (e) {
    // The fetch was aborted (user closed the modal / navigated away). Server-side install continues
    // server-side by design, but for the FE this is a deliberate cancel — no toast needed.
    if (abortController.signal.aborted) return

    // The server's per-machine in-flight guard returns FailedPrecondition with this exact phrase when
    // a concurrent MaintenanceLifecycle is already running for the machine. Render it as a sticky
    // banner so the user has a clear, persistent explanation; other failures fall through to a toast.
    if (typeof e?.message === 'string' && e.message.includes('already in progress')) {
      inProgressFromServer.value = true
    } else {
      showError(`Maintenance install on ${machineId} failed`, e?.message)
    }

    if (progress.value.length > 0) finished.value = true
  } finally {
    installing.value = false
  }
}
</script>

<template>
  <Modal
    v-model:open="open"
    title="Install Talos"
    :action-label="finished ? 'Done' : 'Install'"
    cancel-label="Close"
    :action-disabled="
      machineLoading ||
      !selectedDisk ||
      !selectedVersion ||
      !Object.keys(installVersions).length ||
      inProgressFromServer
    "
    :loading="installing"
    content-class="flex max-w-xl flex-col gap-2"
    @confirm="finished ? (open = false) : doInstall()"
  >
    <template #description>Node {{ machineId }}</template>

    <template v-if="!versionsLoading && machine">
      <TAlert v-if="inProgressFromServer" type="warn" title="Installation in progress">
        A maintenance lifecycle operation is already in progress on this machine. Wait for it to
        finish before starting a new one.
      </TAlert>

      <template v-if="!showProgress">
        <TSelectList v-model="selectedDisk" title="Install Disk" :values="disks" class="self-end" />

        <span v-if="!Object.keys(installVersions).length">No versions found</span>

        <RadioGroup
          id="talos-install-version"
          v-model="selectedVersion"
          class="flex flex-1 flex-col gap-2 overflow-y-auto text-naturals-n13"
        >
          <template v-for="(group, label) in installVersions" :key="label">
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
                  <span v-if="version === currentVersion">(running)</span>
                </div>
              </RadioGroupOption>
            </div>
          </template>
        </RadioGroup>
      </template>

      <div v-else class="flex flex-col gap-2">
        <span class="text-sm">
          The installation runs on the server and continues even if you close this window.
        </span>

        <pre
          ref="progressEl"
          class="flex-1 overflow-y-auto rounded bg-naturals-n2 p-2 text-xs wrap-anywhere whitespace-pre-wrap text-naturals-n11"
          >{{ progress.length ? progress.join('\n') : 'Starting install…' }}</pre
        >
      </div>
    </template>
  </Modal>
</template>
