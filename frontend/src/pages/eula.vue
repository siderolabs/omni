<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed, ref, watchEffect } from 'vue'
import { useRouter } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import { type Resource, ResourceService } from '@/api/grpc'
import type { EulaAcceptanceSpec } from '@/api/omni/specs/auth.pb'
import { DefaultNamespace, EulaAcceptanceID, EulaAcceptanceType } from '@/api/resources'
import TButton from '@/components/Button/TButton.vue'
import TCheckbox from '@/components/Checkbox/TCheckbox.vue'
import PageContainer from '@/components/PageContainer/PageContainer.vue'
import TInput from '@/components/TInput/TInput.vue'
import { eulaAccepted } from '@/methods'
import { useResourceGet } from '@/methods/useResourceGet'
import { showError } from '@/notification'

import { withRuntime, withSkipRequestSignature } from '../api/options'

definePage({
  name: 'Eula',
})

const router = useRouter()

const name = ref('')
const email = ref('')
const accepted = ref(false)
const accepting = ref(false)

const invalidForm = computed(() => !accepted.value || !name.value.trim() || !email.value.trim())

const { data, loading } = useResourceGet<EulaAcceptanceSpec>({
  runtime: Runtime.Omni,
  resource: {
    namespace: DefaultNamespace,
    type: EulaAcceptanceType,
    id: EulaAcceptanceID,
  },
  skipSignature: true,
})

watchEffect(() => {
  if (data.value) router.replace({ name: 'Home' })
})

const accept = async () => {
  if (accepting.value) return

  accepting.value = true

  try {
    await ResourceService.Create<Resource<EulaAcceptanceSpec>>(
      {
        metadata: {
          namespace: DefaultNamespace,
          type: EulaAcceptanceType,
          id: EulaAcceptanceID,
        },
        spec: {
          accepted_by_name: name.value.trim(),
          accepted_by_email: email.value.trim(),
        },
      },
      withRuntime(Runtime.Omni),
      withSkipRequestSignature(),
    )

    eulaAccepted.value = true

    await router.replace('/')
  } catch (e) {
    showError('Failed to accept EULA', e instanceof Error ? e.message : String(e))
  } finally {
    accepting.value = false
  }
}
</script>

<template>
  <PageContainer v-if="!loading && !data" class="flex h-full items-center justify-center">
    <form
      class="flex w-full max-w-2xl flex-col gap-6 rounded-md bg-naturals-n3 px-8 py-8 drop-shadow-md"
      @submit.prevent="accept"
    >
      <h1 class="text-2xl font-bold text-naturals-n13 uppercase">End User License Agreement</h1>

      <div
        class="flex flex-col gap-4 rounded-md bg-naturals-n2 p-4 font-mono text-xs text-naturals-n11"
      >
        <p>
          Before using Sidero Omni, please review the End User License Agreement ("Agreement") at:
          <a
            href="https://siderolabs.com/eula/"
            class="link-primary"
            target="_blank"
            rel="noopener noreferrer"
          >
            https://siderolabs.com/eula/
          </a>
        </p>

        <p>
          By checking the box below and clicking Accept, you agree to the terms of the Agreement on
          behalf of yourself and the organization you represent, confirm that you have authority to
          bind that organization, and agree to ensure that all users of this deployment comply with
          the Agreement's terms.
        </p>
      </div>

      <div class="flex flex-col gap-4">
        <TInput
          v-model="name"
          title="Full Name"
          placeholder="Enter your full name"
          focus
          required
        />

        <TInput
          v-model="email"
          title="Email Address"
          type="email"
          placeholder="Enter your email address"
          required
        />
      </div>

      <TCheckbox
        v-model="accepted"
        label="I have read and agree to the End User License Agreement."
      />

      <TButton
        id="accept-eula"
        class="self-end"
        variant="highlighted"
        type="submit"
        :disabled="accepting || invalidForm"
      >
        Accept
      </TButton>
    </form>
  </PageContainer>
</template>
