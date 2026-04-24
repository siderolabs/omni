<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { compare } from 'semver'
import { computed, onBeforeMount } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import type { TalosVersionSpec } from '@/api/omni/specs/omni.pb'
import type { JoinTokenStatusSpec } from '@/api/omni/specs/siderolink.pb'
import {
  DefaultNamespace,
  DefaultTalosVersion,
  JoinTokenStatusType,
  TalosVersionType,
} from '@/api/resources'
import GrpcTunnelCheckbox from '@/components/GrpcTunnelCheckbox/GrpcTunnelCheckbox.vue'
import Labels from '@/components/Labels/Labels.vue'
import TSelectList from '@/components/SelectList/TSelectList.vue'
import { getDocsLink } from '@/methods'
import { useFeatures } from '@/methods/features'
import { useResourceWatch } from '@/methods/useResourceWatch'
import {
  AUTOMATIC_VERSION,
  type FormState,
  resolveTalosVersion,
} from '@/views/InstallationMedia/useFormState'

definePage({ name: 'InstallationMediaCreateTalosVersion' })

const formState = defineModel<FormState>({ required: true })

const { data: features } = useFeatures()

const { data: talosVersionList, loading: talosVersionsLoading } =
  useResourceWatch<TalosVersionSpec>({
    runtime: Runtime.Omni,
    resource: {
      type: TalosVersionType,
      namespace: DefaultNamespace,
    },
  })

const { data: joinTokenList, loading: joinTokensLoading } = useResourceWatch<JoinTokenStatusSpec>({
  runtime: Runtime.Omni,
  resource: {
    type: JoinTokenStatusType,
    namespace: DefaultNamespace,
  },
})

const talosVersions = computed(() => [
  { label: 'Automatic', value: AUTOMATIC_VERSION },
  ...talosVersionList.value
    .filter((v) => !v.spec.deprecated)
    .map((v) => ({ label: v.spec.version!, value: v.spec.version! }))
    .sort((a, b) => compare(a.value, b.value)),
])

const joinTokens = computed(() => [
  { label: 'Automatic', value: AUTOMATIC_VERSION },
  ...joinTokenList.value
    .toSorted((a, b) => {
      if (a.spec.is_default) return -1
      if (b.spec.is_default) return 1

      return 0
    })
    .map((t) => ({
      label: t.spec.name || t.metadata.id || '',
      value: t.metadata.id || '',
    })),
])

const resolvedTalosVersion = computed(() => resolveTalosVersion(formState.value.talosVersion))

// Form defaults
onBeforeMount(() => {
  formState.value.talosVersion ??= AUTOMATIC_VERSION
  formState.value.joinToken ??= AUTOMATIC_VERSION
})
</script>

<template>
  <div class="flex flex-col items-start gap-4">
    <TSelectList
      v-model="formState.talosVersion"
      :disabled="talosVersionsLoading"
      :values="talosVersions"
      title="Choose Talos Linux Version"
      overhead-title
    />

    <p class="text-xs">
      The latest recommended version of Talos Linux is ({{ DefaultTalosVersion }}).

      <template v-if="features?.spec.talos_pre_release_versions_enabled">
        <br />
        Pre-release versions are suitable for testing purposes but are not advised for production
        environments.
      </template>

      <br />
      Selecting
      <code class="rounded bg-naturals-n4 px-0.5">Automatic</code>
      will automatically get the latest version, even if it changes.
    </p>

    <div class="space-y-2">
      <h2 id="docs-label-id" class="text-xs font-medium text-naturals-n14 after:content-[':']">
        Documentation for Talos Linux {{ resolvedTalosVersion }}
      </h2>
      <ul
        class="list-inside list-disc space-y-2 text-xs text-primary-p3"
        aria-labelledby="docs-label-id"
      >
        <li>
          <a
            :href="
              getDocsLink('talos', `/getting-started/what's-new-in-talos`, {
                talosVersion: resolvedTalosVersion,
              })
            "
            rel="noopener noreferrer"
            target="_blank"
            class="link-primary"
          >
            What's New
          </a>
        </li>

        <li>
          <a
            :href="
              getDocsLink('talos', '/getting-started/support-matrix', {
                talosVersion: resolvedTalosVersion,
              })
            "
            rel="noopener noreferrer"
            target="_blank"
            class="link-primary"
          >
            Support Matrix
          </a>
        </li>
      </ul>
    </div>

    <h2 class="text-base font-medium text-naturals-n14">Omni settings</h2>

    <GrpcTunnelCheckbox v-model="formState.useGrpcTunnel" />

    <TSelectList
      v-model="formState.joinToken"
      :disabled="joinTokensLoading"
      :values="joinTokens"
      title="Join Token"
      overhead-title
    />

    <p class="text-xs">
      Selecting
      <code class="rounded bg-naturals-n4 px-0.5">Automatic</code>
      will automatically get the default join token, even if it changes.
    </p>

    <h3 class="text-sm font-medium text-naturals-n14">Machine User Labels</h3>

    <Labels v-model="formState.machineUserLabels" />
  </div>
</template>
