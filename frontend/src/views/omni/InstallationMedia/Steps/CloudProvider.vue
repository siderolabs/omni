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

const platforms = computed(() =>
  [
    {
      value: 'aws',
      label: 'Amazon Web Services (AWS)',
      description: 'Runs on AWS VMs booted from an AMI',
      documentation: '/platform-specific-installations/cloud-platforms/aws',
    },
    {
      value: 'gcp',
      label: 'Google Cloud (GCP)',
      description: 'Runs on Google Cloud VMs booted from a disk image',
      documentation: '/platform-specific-installations/cloud-platforms/gcp',
    },
    {
      value: 'equinixMetal',
      label: 'Equinix Metal',
      description: 'Runs on Equinix Metal bare-metal servers',
      documentation: '/platform-specific-installations/bare-metal-platforms/equinix-metal',
    },
    {
      value: 'azure',
      label: 'Microsoft Azure',
      description: 'Runs on Microsoft Azure Linux Virtual Machines',
      documentation: '/platform-specific-installations/cloud-platforms/azure',
    },
    {
      value: 'digital-ocean',
      label: 'Digital Ocean',
      description: 'Runs on Digital Ocean droplets',
      documentation: '/platform-specific-installations/cloud-platforms/digitalocean',
    },
    {
      value: 'nocloud',
      label: 'Nocloud',
      description:
        "Runs on various hypervisors supporting 'nocloud' metadata (Proxmox, Oxide Computer, etc.)",
      documentation: '/platform-specific-installations/cloud-platforms/nocloud',
    },
    {
      value: 'openstack',
      label: 'OpenStack',
      description: 'Runs on OpenStack virtual machines',
      documentation: '/platform-specific-installations/cloud-platforms/openstack',
    },
    {
      value: 'vmware',
      label: 'VMWare',
      description: 'Runs on VMWare ESXi virtual machines',
      documentation: '/platform-specific-installations/virtualized-platforms/vmware',
    },
    {
      value: 'akamai',
      label: 'Akamai',
      description: 'Runs on Akamai Cloud (Linode) virtual machines',
      documentation: '/platform-specific-installations/cloud-platforms/akamai',
    },
    {
      value: 'cloudstack',
      label: 'Apache CloudStack',
      description: 'Runs on Apache CloudStack virtual machines',
      documentation: '/platform-specific-installations/cloud-platforms/cloudstack',
    },
    {
      value: 'hcloud',
      label: 'Hetzner',
      description: 'Runs on Hetzner virtual machines',
      documentation: '/platform-specific-installations/cloud-platforms/hetzner',
    },
    {
      value: 'oracle',
      label: 'Oracle Cloud',
      description: 'Runs on Oracle Cloud virtual machines',
      documentation: '/platform-specific-installations/cloud-platforms/oracle',
    },
    {
      value: 'upcloud',
      label: 'UpCloud',
      description: 'Runs on UpCloud virtual machines',
      documentation: '/platform-specific-installations/cloud-platforms/upcloud',
    },
    {
      value: 'vultr',
      label: 'Vultr',
      description: 'Runs on Vultr Cloud Compute virtual machines',
      documentation: '/platform-specific-installations/cloud-platforms/vultr',
    },
    {
      value: 'exoscale',
      label: 'Exoscale',
      description: 'Runs on Exoscale virtual machines',
      documentation: '/platform-specific-installations/cloud-platforms/exoscale',
    },
    {
      value: 'opennebula',
      label: 'OpenNebula',
      description: 'Runs on OpenNebula virtual machines',
      documentation: '/platform-specific-installations/virtualized-platforms/opennebula',
    },
    {
      value: 'scaleway',
      label: 'Scaleway',
      description: 'Runs on Scaleway virtual machines',
      documentation: '/platform-specific-installations/cloud-platforms/scaleway',
    },
  ].map((p) => ({
    ...p,
    documentation: getDocsLink('talos', p.documentation, {
      talosVersion: formState.value.talosVersion,
    }),
  })),
)
</script>

<template>
  <RadioGroup v-mode="formState.cloudPlatform" label="Cloud">
    <RadioGroupOption v-for="platform in platforms" :key="platform.value" :value="platform.value">
      {{ platform.label }}

      <!-- prettier-ignore -->
      <template #description>
        {{ platform.description }} (<a class="text-primary-p3 underline" :href="platform.documentation" @click.stop>documentation</a>).
      </template>
    </RadioGroupOption>
  </RadioGroup>
</template>
