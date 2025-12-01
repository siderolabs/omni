<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import type { PlatformConfigSpec } from '@/api/omni/specs/virtual.pb'
import { CloudPlatformConfigType, DefaultTalosVersion, VirtualNamespace } from '@/api/resources'
import { itemID } from '@/api/watch'
import RadioGroup from '@/components/common/Radio/RadioGroup.vue'
import RadioGroupOption from '@/components/common/Radio/RadioGroupOption.vue'
import { useResourceList } from '@/methods/useResourceList'
import type { FormState } from '@/views/omni/InstallationMedia/InstallationMediaCreate.vue'

const formState = defineModel<FormState>({ required: true })

const { data } = useResourceList<PlatformConfigSpec>({
  runtime: Runtime.Omni,
  resource: {
    namespace: VirtualNamespace,
    type: CloudPlatformConfigType,
  },
})

const platforms = computed(() =>
  data.value?.map((p) => ({
    ...p,
    spec: {
      ...p.spec,
      documentation: `https://www.talos.dev/v${DefaultTalosVersion}/${p.spec.documentation}`,
    },
  })),
)
</script>

<template>
  <RadioGroup v-model="formState.cloudPlatform" label="Cloud">
    <RadioGroupOption
      v-for="platform in platforms"
      :key="itemID(platform)"
      :value="platform.metadata.id!"
    >
      {{ platform.spec.label }}

      <!-- prettier-ignore -->
      <template #description>
        {{ platform.spec.description }} (<a class="link-primary" :href="platform.spec.documentation" @click.stop>documentation</a>).
      </template>
    </RadioGroupOption>
  </RadioGroup>
</template>
