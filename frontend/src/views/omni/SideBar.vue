<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import type { Resource } from '@/api/grpc'
import type { InfraProviderStatusSpec } from '@/api/omni/specs/infra.pb'
import type { MachineStatusMetricsSpec } from '@/api/omni/specs/omni.pb'
import {
  EphemeralNamespace,
  InfraProviderNamespace,
  InfraProviderStatusType,
  MachineStatusMetricsID,
  MachineStatusMetricsType,
} from '@/api/resources'
import Watch from '@/api/watch'
import type { SideBarItem } from '@/components/SideBar/TSideBarList.vue'
import TSidebarList from '@/components/SideBar/TSideBarList.vue'
import { setupBackupStatus } from '@/methods'
import {
  canManageBackupStore,
  canManageUsers,
  canReadClusters,
  canReadMachines,
} from '@/methods/auth'
import { useFeatures } from '@/methods/features'

const machineMetrics = ref<Resource<MachineStatusMetricsSpec>>()
const machineMetricsWatch = new Watch(machineMetrics)

const infraProviderStatuses = ref<Resource<InfraProviderStatusSpec>[]>([])
const infraProvidersWatch = new Watch(infraProviderStatuses)

machineMetricsWatch.setup({
  resource: {
    namespace: EphemeralNamespace,
    type: MachineStatusMetricsType,
    id: MachineStatusMetricsID,
  },
  runtime: Runtime.Omni,
})

infraProvidersWatch.setup(
  computed(() => {
    if (!canReadMachines.value) return

    return {
      resource: {
        namespace: InfraProviderNamespace,
        type: InfraProviderStatusType,
      },
      runtime: Runtime.Omni,
    }
  }),
)

const route = useRoute()

const getRoute = (name: string, path: string) => {
  return route.query.cluster
    ? {
        name: name,
        query: {
          cluster: route.query.cluster,
          namespace: route.query.namespace,
          uid: route.query.uid,
        },
      }
    : path
}

const { status: backupStatus } = setupBackupStatus()

const groupProviders = (
  statuses: Resource<InfraProviderStatusSpec>[],
): Record<string, Resource<InfraProviderStatusSpec>[]> => {
  const res: Record<string, Resource<InfraProviderStatusSpec>[]> = {}

  for (const status of statuses) {
    if (!res[status.spec.name!]) {
      res[status.spec.name!] = []
    }

    res[status.spec.name!].push(status)
  }

  return res
}

const { data: featuresConfig } = useFeatures()

const items = computed(() => {
  const result: SideBarItem[] = [
    {
      name: 'Home',
      route: getRoute('Home', '/'),
      icon: 'home',
    },
  ]

  if (canReadClusters.value) {
    result.push({
      name: 'Clusters',
      route: getRoute('Clusters', '/clusters'),
      icon: 'clusters',
    })
  }

  if (canReadMachines.value) {
    const autoprovisionedMenuItem: SideBarItem = {
      name: 'Auto-Provisioned',
      icon: 'machines-autoprovisioned',
      route: getRoute('MachinesManaged', '/machines/managed'),
    }

    if (infraProviderStatuses.value.length > 0) {
      autoprovisionedMenuItem.subItems = []

      const items = groupProviders(infraProviderStatuses.value)

      for (const name in items) {
        const values = items[name]

        const item: SideBarItem = {
          name: name,
          iconSvgBase64: values[0].spec.icon,
        }

        if (!item.iconSvgBase64) {
          item.icon = 'cloud-connection'
        }

        if (values.length > 1) {
          item.subItems = []

          for (const provider of values) {
            item.subItems.push({
              name: provider.metadata.id!,
              route: getRoute(
                'MachinesManagedProvider',
                `/machines/managed/${provider.metadata.id!}`,
              ),
            })
          }
        } else {
          item.route = getRoute(
            'MachinesManagedProvider',
            `/machines/managed/${values[0].metadata.id}`,
          )
        }

        autoprovisionedMenuItem.subItems.push(item)
      }
    }

    const item: SideBarItem = {
      name: 'Machines',
      route: getRoute('Machines', '/machines'),
      icon: 'nodes',
      subItems: [
        {
          name: 'Self-Managed',
          route: getRoute('MachinesManual', '/machines/manual'),
          icon: 'machines-manual',
        },
        autoprovisionedMenuItem,
      ],
    }

    if (machineMetrics.value?.spec.pending_machines_count) {
      item.subItems!.push({
        name: 'Pending',
        route: getRoute('MachinesPending', '/machines/pending'),
        icon: 'question',
        label: machineMetrics.value.spec.pending_machines_count,
      })
    }

    result.push(item)
  }

  if (canReadMachines.value) {
    result.push({
      name: 'Machine Management',
      icon: 'nodes',
      subItems: [
        {
          name: 'Classes',
          route: getRoute('MachineClasses', '/machine-classes'),
          icon: 'code-bracket',
        },
        {
          name: 'Join Tokens',
          route: getRoute('JoinTokens', '/machine/jointokens'),
          icon: 'key',
        },
      ],
    })
  }

  if (canManageUsers.value || (backupStatus.value.configurable && canManageBackupStore.value)) {
    const subItems: SideBarItem[] = [
      {
        name: 'Users',
        route: getRoute('Users', '/settings/users'),
        icon: 'users',
      },
      {
        name: 'Service Accounts',
        route: getRoute('ServiceAccounts', '/settings/serviceaccounts'),
        icon: 'users',
      },
      {
        name: 'Infra Providers',
        route: getRoute('InfraProviders', '/settings/infraproviders'),
        icon: 'machines-autoprovisioned',
      },
      {
        name: 'Backups',
        route: getRoute('Backups', '/settings/backups'),
        icon: 'rollback',
      },
    ]

    if (featuresConfig.value?.spec.stripe_settings?.enabled) {
      subItems.push({
        name: 'Stripe',
        route: 'https://billing.stripe.com/p/login/8wMcOC8z51GgdPi144',
        regularLink: true,
        icon: 'dashboard',
      })
    }

    result.push({
      name: 'Settings',
      icon: 'settings',
      subItems,
    })
  }

  return result
})
</script>

<template>
  <div>
    <TSidebarList :items="items" />

    <RouterView name="clusterSidebar" class="border-t border-naturals-n4" />
  </div>
</template>
