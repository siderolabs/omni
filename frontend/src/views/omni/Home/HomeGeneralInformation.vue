<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { useClipboard } from '@vueuse/core'
import { onBeforeMount, ref } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import type { DefaultJoinTokenSpec, SiderolinkAPIConfigSpec } from '@/api/omni/specs/siderolink.pb'
import type { SysVersionSpec } from '@/api/omni/specs/system.pb'
import {
  APIConfigType,
  ConfigID,
  DefaultJoinTokenID,
  DefaultJoinTokenType,
  DefaultNamespace,
  EphemeralNamespace,
  SysVersionID,
  SysVersionType,
} from '@/api/resources'
import TButton from '@/components/common/Button/TButton.vue'
import Card from '@/components/common/Card/Card.vue'
import TSpinner from '@/components/common/Spinner/TSpinner.vue'
import TAlert from '@/components/TAlert.vue'
import {
  downloadAuditLog,
  downloadMachineJoinConfig,
  downloadOmniconfig,
  downloadTalosconfig,
  getKernelArgs,
} from '@/methods'
import { canReadAuditLog } from '@/methods/auth'
import { auditLogEnabled } from '@/methods/features'
import { useResourceWatch } from '@/methods/useResourceWatch'
import HomeGeneralInformationCopyable from '@/views/omni/Home/HomeGeneralInformationCopyable.vue'

const auditLogAvailable = ref(false)
const { copy, copied } = useClipboard()

onBeforeMount(async () => {
  auditLogAvailable.value = await auditLogEnabled()
})

async function copyKernelArgs() {
  copy(await getKernelArgs())
}

const { data: sysData } = useResourceWatch<SysVersionSpec>({
  resource: {
    type: SysVersionType,
    namespace: EphemeralNamespace,
    id: SysVersionID,
  },
  runtime: Runtime.Omni,
})

const { data: joinTokenData } = useResourceWatch<DefaultJoinTokenSpec>({
  resource: {
    type: DefaultJoinTokenType,
    namespace: DefaultNamespace,
    id: DefaultJoinTokenID,
  },
  runtime: Runtime.Omni,
})

const {
  data: apiConfigData,
  err: apiConfigErr,
  loading: apiConfigLoading,
} = useResourceWatch<SiderolinkAPIConfigSpec>({
  resource: {
    type: APIConfigType,
    namespace: DefaultNamespace,
    id: ConfigID,
  },
  runtime: Runtime.Omni,
})
</script>

<template>
  <Card class="flex flex-col gap-6 p-4 text-naturals-n14">
    <header class="flex items-center justify-between">
      <h2 class="text-sm font-medium">General Information</h2>
      <TSpinner v-if="apiConfigLoading" class="size-4" />
    </header>

    <TAlert v-if="apiConfigErr" type="error" :title="apiConfigErr" />

    <dl class="flex flex-col gap-4">
      <HomeGeneralInformationCopyable
        title="Backend Version"
        :value="sysData?.spec.backend_version"
      />

      <HomeGeneralInformationCopyable
        title="API Endpoint"
        :value="apiConfigData?.spec.machine_api_advertised_url"
      />

      <HomeGeneralInformationCopyable
        title="SideroLink Endpoint"
        :value="apiConfigData?.spec.wireguard_advertised_endpoint"
      />

      <HomeGeneralInformationCopyable
        title="Join Token"
        secret
        :value="joinTokenData?.spec.token_id"
      />
    </dl>

    <hr class="border border-naturals-n4" />

    <section class="flex flex-col gap-2">
      <h3 class="text-sm font-medium">Add Machines</h3>

      <TButton
        is="router-link"
        icon="long-arrow-down"
        icon-position="left"
        :to="{ name: 'InstallationMedia' }"
      >
        Download Installation Media
      </TButton>

      <TButton icon="long-arrow-down" icon-position="left" @click="downloadMachineJoinConfig()">
        Download Machine Join Config
      </TButton>

      <TButton :icon="copied ? 'check' : 'copy'" icon-position="left" @click="copyKernelArgs">
        Copy Kernel Parameters
      </TButton>
    </section>

    <section class="flex flex-col gap-2">
      <h3 class="text-sm font-medium">CLI</h3>

      <TButton type="primary" icon="document" icon-position="left" @click="downloadTalosconfig()">
        Download
        <code>talosconfig</code>
      </TButton>

      <TButton
        type="primary"
        icon="talos-config"
        icon-position="left"
        @click="$router.push({ query: { modal: 'downloadTalosctlBinaries' } })"
      >
        Download talosctl
      </TButton>

      <TButton type="primary" icon="document" icon-position="left" @click="downloadOmniconfig">
        Download
        <code>omniconfig</code>
      </TButton>

      <TButton
        type="primary"
        icon="talos-config"
        icon-position="left"
        @click="$router.push({ query: { modal: 'downloadOmnictlBinaries' } })"
      >
        Download omnictl
      </TButton>
    </section>

    <section v-if="canReadAuditLog && auditLogAvailable" class="flex flex-col gap-2">
      <h3 class="text-sm font-medium">Tools</h3>

      <TButton type="primary" icon="document" icon-position="left" @click="downloadAuditLog">
        Get audit logs
      </TButton>
    </section>
  </Card>
</template>
