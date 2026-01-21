<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed, watch } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import type { SiderolinkAPIConfigSpec } from '@/api/omni/specs/siderolink.pb'
import { APIConfigType, ConfigID, DefaultNamespace } from '@/api/resources'
import TCheckbox from '@/components/common/Checkbox/TCheckbox.vue'
import Tooltip from '@/components/common/Tooltip/Tooltip.vue'
import { useResourceGet } from '@/methods/useResourceGet'

const model = defineModel<boolean>()

const { data: siderolinkAPIConfig } = useResourceGet<SiderolinkAPIConfigSpec>({
  runtime: Runtime.Omni,
  resource: {
    namespace: DefaultNamespace,
    type: APIConfigType,
    id: ConfigID,
  },
})
const enforceGrpcTunnel = computed(
  () => siderolinkAPIConfig.value?.spec.enforce_grpc_tunnel ?? false,
)

watch(
  enforceGrpcTunnel,
  (enforced) => {
    if (enforced) {
      model.value = true
    }
  },
  { immediate: true },
)
</script>

<template>
  <Tooltip>
    <TCheckbox
      v-model="model"
      :disabled="enforceGrpcTunnel"
      label="Tunnel Omni management traffic over HTTP/2"
    />

    <template #description>
      <div class="flex flex-col gap-1 p-2">
        <p>
          Configure Talos to use the SideroLink (WireGuard) gRPC tunnel over HTTP/2 for Omni
          management traffic, instead of UDP. Only enable this if the network blocks UDP packets, as
          HTTP tunneling adds significant overhead for communications.
        </p>
        <p v-if="enforceGrpcTunnel">
          As it is enabled in Omni on instance-level, it cannot be disabled for the installation
          media.
        </p>
      </div>
    </template>
  </Tooltip>
</template>
