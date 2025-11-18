<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed } from 'vue'

import RadioGroup from '@/components/common/Radio/RadioGroup.vue'
import RadioGroupOption from '@/components/common/Radio/RadioGroupOption.vue'
import { getDocsLink } from '@/methods'
import type { FormState } from '@/views/omni/InstallationMedia/InstallationMediaCreate.vue'

const formState = defineModel<FormState>({ required: true })

const SBCs = computed(() =>
  [
    {
      value: 'rpi_generic',
      label: 'Raspberry Pi Series',
      documentation: '/platform-specific-installations/single-board-computers/rpi_generic',
    },
    {
      value: 'revpi_generic',
      label: 'Revolution Pi Series',
    },
    {
      value: 'bananapi_m64',
      label: 'Banana Pi M64',
      documentation: '/platform-specific-installations/single-board-computers/bananapi_m64',
    },
    {
      value: 'nanopi_r4s',
      label: 'Friendlyelec Nano PI R4S',
      documentation: '/platform-specific-installations/single-board-computers/nanopi_r4s',
    },
    {
      value: 'nanopi_r5s',
      label: 'Friendlyelec Nano PI R5S',
    },
    {
      value: 'jetson_nano',
      label: 'Jetson Nano',
      documentation: '/platform-specific-installations/single-board-computers/jetson_nano',
    },
    {
      value: 'libretech_all_h3_cc_h5',
      label: 'Libre Computer Board ALL-H3-CC',
      documentation:
        '/platform-specific-installations/single-board-computers/libretech_all_h3_cc_h5',
    },
    {
      value: 'orangepi_r1_plus_lts',
      label: 'Orange Pi R1 Plus LTS',
      documentation: '/platform-specific-installations/single-board-computers/orangepi_r1_plus_lts',
    },
    {
      value: 'pine64',
      label: 'Pine64',
      documentation: '/platform-specific-installations/single-board-computers/pine64',
    },
    {
      value: 'rock64',
      label: 'Pine64 Rock64',
      documentation: '/platform-specific-installations/single-board-computers/rock64',
    },
    {
      value: 'rock4cplus',
      label: 'Radxa ROCK 4C Plus',
      documentation: '/platform-specific-installations/single-board-computers/rock4cplus',
    },
    {
      value: 'rock4se',
      label: 'Radxa ROCK 4SE',
    },
    {
      value: 'rock5a',
      label: 'Radxa ROCK 5A',
      documentation: '/platform-specific-installations/single-board-computers/rock5b',
    },
    {
      value: 'rock5b',
      label: 'Radxa ROCK 5B',
      documentation: '/platform-specific-installations/single-board-computers/rock5b',
    },
    {
      value: 'rockpi_4',
      label: 'Radxa ROCK PI 4',
      documentation: '/platform-specific-installations/single-board-computers/rockpi_4',
    },
    {
      value: 'rockpi_4c',
      label: 'Radxa ROCK PI 4C',
      documentation: '/platform-specific-installations/single-board-computers/rockpi_4c',
    },
    {
      value: 'helios64',
      label: 'Kobol Helios64',
    },
    {
      value: 'turingrk1',
      label: 'Turing RK1',
      documentation: '/platform-specific-installations/single-board-computers/turing_rk1',
    },
    {
      value: 'orangepi-5',
      label: 'Orange Pi 5',
      documentation: '/platform-specific-installations/single-board-computers/orangepi_5',
    },
    {
      value: 'orangepi-5-plus',
      label: 'Orange Pi 5 Plus',
    },
    {
      value: 'rockpro64',
      label: 'Pine64 RockPro64',
    },
  ].map((sbc) => ({
    ...sbc,
    documentation:
      sbc.documentation &&
      getDocsLink('talos', sbc.documentation, { talosVersion: formState.value.talosVersion }),
  })),
)
</script>

<template>
  <RadioGroup v-mode="formState.sbcType" label="Single Board Computer">
    <RadioGroupOption v-for="sbc in SBCs" :key="sbc.value" :value="sbc.value">
      {{ sbc.label }}

      <template #description>
        <span :class="!sbc.documentation && 'after:content-[\'.\']'">Runs on {{ sbc.label }}</span>

        <!-- prettier-ignore -->
        <template v-if="sbc.documentation">
          (<a class="text-primary-p3 underline" :href="sbc.documentation" @click.stop>documentation</a>).
        </template>
      </template>
    </RadioGroupOption>
  </RadioGroup>
</template>
