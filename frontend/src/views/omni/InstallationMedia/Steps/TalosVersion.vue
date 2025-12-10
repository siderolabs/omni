<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { watchOnce } from '@vueuse/core'
import { compare } from 'semver'
import { computed, onBeforeMount, watch } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import type { TalosVersionSpec } from '@/api/omni/specs/omni.pb'
import type { JoinTokenStatusSpec, SiderolinkAPIConfigSpec } from '@/api/omni/specs/siderolink.pb'
import {
  APIConfigType,
  ConfigID,
  DefaultNamespace,
  DefaultTalosVersion,
  JoinTokenStatusType,
  TalosVersionType,
} from '@/api/resources'
import TCheckbox from '@/components/common/Checkbox/TCheckbox.vue'
import TSelectList from '@/components/common/SelectList/TSelectList.vue'
import Tooltip from '@/components/common/Tooltip/Tooltip.vue'
import { getDocsLink } from '@/methods'
import { useFeatures } from '@/methods/features'
import { useResourceGet } from '@/methods/useResourceGet'
import { useResourceWatch } from '@/methods/useResourceWatch'
import type { FormState } from '@/views/omni/InstallationMedia/InstallationMediaCreate.vue'

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

const { data: siderolinkAPIConfig } = useResourceGet<SiderolinkAPIConfigSpec>({
  runtime: Runtime.Omni,
  resource: {
    namespace: DefaultNamespace,
    type: APIConfigType,
    id: ConfigID,
  },
})

const talosVersions = computed(() =>
  talosVersionList.value
    .filter((v) => !v.spec.deprecated)
    .map((v) => v.spec.version!)
    .sort(compare),
)

const joinTokens = computed(() => joinTokenList.value.map((t) => t.metadata.id ?? ''))
const enforceGrpcTunnel = computed(
  () => siderolinkAPIConfig.value?.spec.enforce_grpc_tunnel ?? false,
)

watch(
  enforceGrpcTunnel,
  (enforced) => {
    if (enforced) {
      formState.value.useGrpcTunnel = true
    }
  },
  { immediate: true },
)

// Form defaults
onBeforeMount(() => (formState.value.talosVersion ??= DefaultTalosVersion))
watchOnce(joinTokens, (v) => (formState.value.joinToken ??= v[0]))
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
      We strongly recommend using the latest stable version of Talos Linux ({{
        DefaultTalosVersion
      }}).
      <template v-if="features?.spec.talos_pre_release_versions_enabled">
        <br />
        Pre-release versions are suitable for testing purposes but are not advised for production
        environments.
      </template>
    </p>

    <div class="space-y-2">
      <h2 id="docs-label-id" class="text-xs font-medium text-naturals-n14 after:content-[':']">
        Documentation for Talos Linux {{ formState.talosVersion }}
      </h2>
      <ul
        class="list-inside list-disc space-y-2 text-xs text-primary-p3"
        aria-labelledby="docs-label-id"
      >
        <li>
          <a
            :href="
              getDocsLink('talos', `/getting-started/what's-new-in-talos`, {
                talosVersion: formState.talosVersion,
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
                talosVersion: formState.talosVersion,
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

    <h2 class="text-sm font-medium text-naturals-n14">Omni settings</h2>

    <TSelectList
      v-model="formState.joinToken"
      :disabled="joinTokensLoading"
      :values="joinTokens"
      title="Join Token"
      overhead-title
    />

    <Tooltip>
      <TCheckbox
        v-model="formState.useGrpcTunnel"
        :disabled="enforceGrpcTunnel"
        label="Tunnel Omni management traffic over HTTP2"
      />

      <template #description>
        <div class="flex flex-col gap-1 p-2">
          <p>
            Configure Talos to use the SideroLink (WireGuard) gRPC tunnel over HTTP2 for Omni
            management traffic, instead of UDP. Note that this will add overhead to the traffic.
          </p>
          <p v-if="enforceGrpcTunnel">
            As it is enabled in Omni on instance-level, it cannot be disabled for the installation
            media.
          </p>
        </div>
      </template>
    </Tooltip>
  </div>
</template>
