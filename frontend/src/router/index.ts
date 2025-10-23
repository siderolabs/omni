// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

import { authGuard } from '@auth0/auth0-vue'
import { Userpilot } from 'userpilot'
import type { RouteRecordRaw } from 'vue-router'
import { createRouter, createWebHistory, RouterView } from 'vue-router'

import { AuthFlowQueryParam, RedirectQueryParam } from '@/api/resources'
import { current } from '@/context'
import { AuthType, authType } from '@/methods'
import { loadCurrentUser } from '@/methods/auth'
import { getAuthCookies, isAuthorized } from '@/methods/key'
import { MachineFilterOption } from '@/methods/machine'
import { refreshTitle } from '@/methods/title'

export const FrontendAuthFlow = 'frontend'
const requireCookies = false

export const routes: RouteRecordRaw[] = [
  // Unauthenticated routes
  { path: '/forbidden', component: () => import('@/views/common/Forbidden.vue') },
  { path: '/badrequest', component: () => import('@/views/common/BadRequest.vue') },
  { path: '/:catchAll(.*)', component: () => import('@/views/common/PageNotFound.vue') },
  {
    path: '/authenticate',
    name: 'Authenticate',
    component: () => import('@/views/omni/Auth/Authenticate.vue'),
    beforeEnter: async (to) => {
      return authType.value === AuthType.Auth0 ? await authGuard(to) : true
    },
  },
  {
    path: '/oidc-login/:authRequestId',
    name: 'OIDC Login',
    component: () => import('@/views/omni/Auth/OIDC.vue'),
    beforeEnter: async (to) => {
      if (await isAuthorized()) {
        return true
      }

      return {
        name: 'Authenticate',
        query: { [AuthFlowQueryParam]: FrontendAuthFlow, [RedirectQueryParam]: to.fullPath },
      }
    },
  },

  // Redirects for legacy routes
  { path: '/omni', redirect: '/' },
  { path: '/omni/:catchAll(.*)', redirect: (to) => `/${to.params.catchAll}` },
  { path: '/cluster', redirect: '/clusters' },
  { path: '/cluster/:catchAll(.*)', redirect: (to) => `/clusters/${to.params.catchAll}` },
  { path: '/machine', redirect: '/machines' },
  { path: '/machine/:catchAll(.*)', redirect: (to) => `/machines/${to.params.catchAll}` },

  // Authenticated routes
  {
    path: '/',
    components: {
      default: RouterView,
      sidebar: () => import('@/components/SideBar/TSideBar.vue'),
    },
    beforeEnter: async (to) => {
      let authorized = await isAuthorized()

      if (requireCookies && !getAuthCookies()) {
        authorized = false
      }

      if (authorized) {
        await loadCurrentUser()
      }

      if (authorized) {
        await refreshTitle()

        return true
      }

      return {
        name: 'Authenticate',
        query: { [AuthFlowQueryParam]: FrontendAuthFlow, [RedirectQueryParam]: to.fullPath },
      }
    },
    children: [
      {
        path: '',
        name: 'Home',
        component: () => import('@/views/omni/Home/Home.vue'),
      },
      {
        path: 'clusters',
        children: [
          {
            path: '',
            name: 'Clusters',
            component: () => import('@/views/omni/Clusters/Clusters.vue'),
          },
          {
            path: 'create',
            name: 'ClusterCreate',
            component: () => import('@/views/omni/Clusters/Management/ClusterCreate.vue'),
          },
          {
            path: ':cluster',
            component: () => import('@/views/cluster/ClusterScoped.vue'),
            children: [
              {
                path: '',
                name: 'ClusterOverview',
                component: () => import('@/views/cluster/Overview/Overview.vue'),
              },
              {
                path: 'nodes',
                name: 'Nodes',
                component: () => import('@/views/cluster/Nodes/NodesList.vue'),
              },
              {
                path: 'scale',
                name: 'ClusterScale',
                component: () => import('@/views/omni/Clusters/Management/ClusterScale.vue'),
              },
              {
                path: 'pods',
                name: 'Pods',
                component: () => import('@/views/cluster/Pods/TPods.vue'),
              },
              {
                path: 'patches',
                name: 'ClusterConfigPatches',
                component: () => import('@/views/cluster/Config/ClusterPatches.vue'),
              },
              {
                path: 'patches/:patch',
                name: 'ClusterPatchEdit',
                component: () => import('@/views/cluster/Config/PatchEdit.vue'),
              },
              {
                path: 'manifests',
                name: 'KubernetesManifestSync',
                component: () => import('@/views/cluster/Manifest/Sync.vue'),
              },
              {
                path: 'backups',
                name: 'Backups',
                component: () => import('@/views/cluster/Backups/Backups.vue'),
              },
              {
                path: 'machine/:machine',
                children: [
                  {
                    path: 'patches/:patch',
                    name: 'ClusterMachinePatchEdit',
                    component: () => import('@/views/cluster/Config/PatchEdit.vue'),
                  },
                  {
                    path: '',
                    name: 'NodeDetails',
                    component: () => import('@/views/cluster/Nodes/NodeDetails.vue'),
                    children: [
                      {
                        path: '',
                        name: 'NodeOverview',
                        component: () => import('@/views/cluster/Nodes/NodeOverview.vue'),
                      },
                      {
                        path: 'monitor',
                        name: 'NodeMonitor',
                        component: () => import('@/views/cluster/Nodes/NodeMonitor.vue'),
                      },
                      {
                        path: 'logs/:service',
                        name: 'NodeLogs',
                        component: () => import('@/views/cluster/Nodes/NodeLogs.vue'),
                      },
                      {
                        path: 'config',
                        name: 'NodeConfig',
                        component: () => import('@/views/cluster/Nodes/NodeConfig.vue'),
                      },
                      {
                        path: 'patches',
                        name: 'NodePatches',
                        component: () => import('@/views/cluster/Nodes/NodePatches.vue'),
                      },
                      {
                        path: 'mounts',
                        name: 'NodeMounts',
                        component: () => import('@/views/cluster/Nodes/NodeMounts.vue'),
                      },
                      {
                        path: 'extensions',
                        name: 'NodeExtensions',
                        component: () => import('@/views/cluster/Nodes/NodeExtensions.vue'),
                      },
                    ],
                  },
                ],
              },
            ],
          },
        ],
      },
      {
        path: 'machines',
        children: [
          {
            path: '',
            name: 'Machines',
            component: () => import('@/views/omni/Machines/Machines.vue'),
          },
          {
            path: 'manual',
            name: 'MachinesManual',
            component: () => import('@/views/omni/Machines/Machines.vue'),
            props: {
              filter: MachineFilterOption.Manual,
            },
          },
          {
            path: 'managed',
            children: [
              {
                path: '',
                name: 'MachinesManaged',
                component: () => import('@/views/omni/Machines/Machines.vue'),
                props: {
                  filter: MachineFilterOption.Managed,
                },
              },
              {
                path: ':provider',
                name: 'MachinesManagedProvider',
                component: () => import('@/views/omni/Machines/Machines.vue'),
              },
            ],
          },
          {
            path: 'pending',
            name: 'MachinesPending',
            component: () => import('@/views/omni/Machines/MachinesPending.vue'),
          },
          {
            path: 'installation-media',
            name: 'InstallationMedia',
            component: () => import('@/views/omni/InstallationMedia/InstallationMedia.vue'),
          },
          {
            path: 'jointokens',
            name: 'JoinTokens',
            component: () => import('@/views/omni/Settings/JoinTokens.vue'),
          },
          {
            path: ':machine',
            name: 'Machine',
            component: () => import('@/views/omni/Machines/Machine.vue'),
            redirect: {
              name: 'MachineLogs',
            },
            children: [
              {
                path: 'logs',
                name: 'MachineLogs',
                component: () => import('@/views/omni/Machines/MachineLogs.vue'),
              },
              {
                path: 'patches',
                children: [
                  {
                    path: '',
                    name: 'MachineConfigPatches',
                    component: () => import('@/views/omni/Machines/MachinePatches.vue'),
                  },
                  {
                    path: ':patch',
                    name: 'MachinePatchEdit',
                    component: () => import('@/views/cluster/Config/PatchEdit.vue'),
                  },
                ],
              },
            ],
          },
        ],
      },
      {
        path: 'machine-classes',
        children: [
          {
            path: '',
            name: 'MachineClasses',
            component: () => import('@/views/omni/MachineClasses/MachineClasses.vue'),
          },
          {
            path: 'create',
            name: 'MachineClassCreate',
            component: () => import('@/views/omni/MachineClasses/MachineClass.vue'),
          },
          {
            path: ':classname',
            name: 'MachineClassEdit',
            component: () => import('@/views/omni/MachineClasses/MachineClass.vue'),
            props: {
              edit: true,
            },
          },
        ],
      },
      {
        path: 'settings',
        name: 'Settings',
        component: () => import('@/views/omni/Settings/Settings.vue'),
        redirect: {
          name: 'Users',
        },
        children: [
          {
            path: 'users',
            name: 'Users',
            component: () => import('@/views/omni/Users/Users.vue'),
            meta: {
              title: 'Users',
            },
          },
          {
            path: 'serviceaccounts',
            name: 'ServiceAccounts',
            component: () => import('@/views/omni/Users/ServiceAccounts.vue'),
            meta: {
              title: 'Service Accounts',
            },
          },
          {
            path: 'infraproviders',
            name: 'InfraProviders',
            component: () => import('@/views/omni/Settings/InfraProviders.vue'),
            meta: {
              title: 'Infra Providers',
            },
          },
          {
            path: 'backups',
            name: 'BackupStorage',
            component: () => import('@/views/omni/Settings/BackupStorage.vue'),
            meta: {
              title: 'Backup Storage',
            },
          },
        ],
      },
    ],
  },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

router.beforeEach((to) => {
  if (!to.params.cluster) {
    return true
  }

  current.value = to.params.cluster as string

  return true
})

router.afterEach(() => {
  Userpilot.reload()
})

export default router
