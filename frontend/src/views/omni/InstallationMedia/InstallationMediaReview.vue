<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { useRoute } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import type { InstallationMediaConfigSpec } from '@/api/omni/specs/omni.pb'
import { DefaultNamespace, InstallationMediaConfigType } from '@/api/resources'
import PageHeader from '@/components/common/PageHeader.vue'
import TSpinner from '@/components/common/Spinner/TSpinner.vue'
import TAlert from '@/components/TAlert.vue'
import { useResourceWatch } from '@/methods/useResourceWatch'
import { presetToFormState } from '@/views/omni/InstallationMedia/formStateToPreset'
import Confirmation from '@/views/omni/InstallationMedia/Steps/Confirmation.vue'

const route = useRoute()

const {
  data: preset,
  loading: presetLoading,
  err,
} = useResourceWatch<InstallationMediaConfigSpec>(() => ({
  skip: !route.params.presetId,
  runtime: Runtime.Omni,
  resource: {
    namespace: DefaultNamespace,
    type: InstallationMediaConfigType,
    id: route.params.presetId as string,
  },
}))
</script>

<template>
  <div>
    <PageHeader title="Previously Created Media" />
    <TSpinner v-if="presetLoading" class="mx-auto size-8" />

    <Confirmation
      v-else-if="preset"
      is-review-page
      :saved-preset="preset"
      :model-value="presetToFormState(preset.spec)"
    />

    <TAlert v-else-if="err" title="Error" type="error">
      {{ err }}
    </TAlert>
    <TAlert v-else title="Not Found" type="error">
      The
      <code>InstallationMediaConfigType</code>
      {{ route.params.presetId }} does not exist
    </TAlert>
  </div>
</template>
