<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { gte } from 'semver'
import { computed } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import type { PlatformConfigSpec } from '@/api/omni/specs/virtual.pb'
import { CloudPlatformConfigType, VirtualNamespace } from '@/api/resources'
import { itemID } from '@/api/watch'
import RadioGroup from '@/components/common/Radio/RadioGroup.vue'
import RadioGroupOption from '@/components/common/Radio/RadioGroupOption.vue'
import { getLegacyDocsLink } from '@/methods'
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
  data.value
    ?.filter(
      (p) =>
        !p.spec.min_version ||
        (formState.value.talosVersion && gte(formState.value.talosVersion, p.spec.min_version)),
    )
    .map((p) => ({
      ...p,
      spec: {
        ...p.spec,
        documentation: p.spec.documentation && getLegacyDocsLink(p.spec.documentation),
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

      <template #description>
        <span :class="!platform.spec.documentation && 'after:content-[\'.\']'">
          {{ platform.spec.description }}
        </span>

        <!-- prettier-ignore -->
        <template v-if="platform.spec.documentation">
          (<a class="link-primary" :href="platform.spec.documentation" @click.stop>documentation</a>).
        </template>
      </template>
    </RadioGroupOption>
  </RadioGroup>
</template>
