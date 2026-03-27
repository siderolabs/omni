<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { useLocalStorage } from '@vueuse/core'

import { Runtime } from '@/api/common/omni.pb'
import type { MachineStatusMetricsSpec } from '@/api/omni/specs/omni.pb'
import {
  EphemeralNamespace,
  MachineStatusMetricsID,
  MachineStatusMetricsType,
} from '@/api/resources'
import TButton from '@/components/Button/TButton.vue'
import CodeBlock from '@/components/CodeBlock/CodeBlock.vue'
import { getDocsLink } from '@/methods'
import { useResourceWatch } from '@/methods/useResourceWatch'

const isDismissed = useLocalStorage('_add_first_machine_tutorial_dismissed', false)

const { data, loading } = useResourceWatch<MachineStatusMetricsSpec>(() => ({
  skip: isDismissed.value,
  resource: {
    namespace: EphemeralNamespace,
    type: MachineStatusMetricsType,
    id: MachineStatusMetricsID,
  },
  runtime: Runtime.Omni,
}))

const awsCode = `aws ec2 run-instances \\
    --image-id $(aws ec2 describe-images --owners 540036508848     --filters "Name=architecture,Values=x86_64"     --query 'Images | sort_by(@, &CreationDate) | reverse(@) | [?!contains(Name, \`beta\`) && !contains(Name, \`alpha\`) && !contains(Name, \`Beta\`) && !contains(Name, \`Alpha\`)] | [0].[ImageId]'     --output text) \\
    --count 1 \\
    --instance-type t3.small   \\
    --associate-public-ip-address \\
    --user-data "$(omnictl jointoken machine-config)"`

const pxeBootCode =
  'docker run -t --network host ghcr.io/siderolabs/booter:v0.3.0 -k "$(omnictl jointoken kernel-args)"'
</script>

<template>
  <section
    v-if="
      !isDismissed &&
      !loading &&
      !data?.spec.registered_machines_count &&
      !data?.spec.pending_machines_count
    "
    class="space-y-4 rounded-lg border border-primary-p3 bg-naturals-n2 p-6"
  >
    <header>
      <h2 class="text-sm font-medium text-naturals-n14">Getting Started: Machines</h2>
    </header>

    <div class="space-y-4 text-xs font-medium">
      <p>To add your first machine create Installation Media and put it onto your machine.</p>

      <div class="space-y-2">
        <p>Local testing</p>
        <CodeBlock
          code="talosctl cluster create qemu --omni-api-endpoint $(omnictl jointoken omni-endpoint) --workers 0 --controlplanes 3"
        ></CodeBlock>

        <p>AWS</p>
        <CodeBlock :code="awsCode"></CodeBlock>

        <p>PXE boot</p>
        <CodeBlock :code="pxeBootCode"></CodeBlock>

        <p>Download ISO</p>
        <CodeBlock code="omnictl download iso --arch amd64"></CodeBlock>
      </div>

      <p>
        Read
        <a
          class="link-primary"
          :href="
            getDocsLink(
              'omni',
              '/omni-cluster-setup/registering-machines/register-machines-with-omni',
            )
          "
          target="_blank"
          rel="noopener noreferrer"
        >
          Register machines with Omni
        </a>
        documentation to learn about other options.
      </p>
    </div>

    <div class="grid grid-cols-2 gap-2">
      <TButton icon="close" icon-position="left" @click="isDismissed = true">Dismiss</TButton>

      <TButton
        is="router-link"
        icon="long-arrow-down"
        icon-position="left"
        :to="{ name: 'InstallationMedia' }"
      >
        Download Installation Media
      </TButton>
    </div>
  </section>
</template>
