<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import type { SBCConfigSpec } from '@/api/omni/specs/virtual.pb'
import { SBCConfigType, VirtualNamespace } from '@/api/resources'
import { itemID } from '@/api/watch'
import RadioGroup from '@/components/common/Radio/RadioGroup.vue'
import RadioGroupOption from '@/components/common/Radio/RadioGroupOption.vue'
import { getLegacyDocsLink } from '@/methods'
import { useResourceList } from '@/methods/useResourceList'
import type { FormState } from '@/views/omni/InstallationMedia/InstallationMediaCreate.vue'

const formState = defineModel<FormState>({ required: true })

const { data } = useResourceList<SBCConfigSpec>({
  runtime: Runtime.Omni,
  resource: {
    namespace: VirtualNamespace,
    type: SBCConfigType,
  },
})

const SBCs = computed(() =>
  data.value?.map((sbc) => ({
    ...sbc,
    spec: {
      ...sbc.spec,
      documentation: sbc.spec.documentation && getLegacyDocsLink(sbc.spec.documentation),
    },
  })),
)
</script>

<template>
  <RadioGroup v-model="formState.sbcType" label="Single Board Computer">
    <RadioGroupOption v-for="sbc in SBCs" :key="itemID(sbc)" :value="sbc.metadata.id!">
      {{ sbc.spec.label }}

      <template #description>
        <span :class="!sbc.spec.documentation && 'after:content-[\'.\']'">
          Runs on {{ sbc.spec.label }}
        </span>

        <!-- prettier-ignore -->
        <template v-if="sbc.spec.documentation">
          (<a class="link-primary" :href="sbc.spec.documentation" @click.stop>documentation</a>).
        </template>
      </template>
    </RadioGroupOption>
  </RadioGroup>
</template>
