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
import PageContainer from '@/components/common/PageContainer/PageContainer.vue'
import PageHeader from '@/components/common/PageHeader.vue'
import TSpinner from '@/components/common/Spinner/TSpinner.vue'
import TAlert from '@/components/TAlert.vue'
import { useResourceWatch } from '@/methods/useResourceWatch'
import Confirmation from '@/views/omni/InstallationMedia/Confirmation.vue'
import { presetToFormState } from '@/views/omni/InstallationMedia/formStateToPreset'

definePage({ name: 'InstallationMediaReview' })

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
  <PageContainer>
    <PageHeader :title="`Previously saved preset &quot;${route.params.presetId}&quot;`" />
    <TSpinner v-if="presetLoading" class="mx-auto size-8" />

    <Confirmation v-else-if="preset" is-review-page :model-value="presetToFormState(preset.spec)" />

    <TAlert v-else-if="err" title="Error" type="error">
      {{ err }}
    </TAlert>
    <TAlert v-else title="Not Found" type="error">
      The
      <code>InstallationMediaConfigType</code>
      {{ route.params.presetId }} does not exist
    </TAlert>
  </PageContainer>
</template>
