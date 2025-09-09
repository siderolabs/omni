// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

import { authGuard } from '@auth0/auth0-vue'
import { Userpilot } from 'userpilot'
import type { RouteRecordRaw } from 'vue-router'
import { createRouter, createWebHistory, RouterView } from 'vue-router'

import { current } from '@/context'
import { AuthType, authType } from '@/methods'
import { loadCurrentUser } from '@/methods/auth'
import { getAuthCookies, isAuthorized } from '@/methods/key'
import { MachineFilterOption } from '@/methods/machine'
import { refreshTitle } from '@/methods/title'
import ClusterBackups from '@/views/cluster/Backups/Backups.vue'
import ClusterScoped from '@/views/cluster/ClusterScoped.vue'
import ClusterPatches from '@/views/cluster/Config/ClusterPatches.vue'
import PatchEdit from '@/views/cluster/Config/PatchEdit.vue'
import KubernetesManifestSync from '@/views/cluster/Manifest/Sync.vue'
import NodeConfig from '@/views/cluster/Nodes/NodeConfig.vue'
import NodeDetails from '@/views/cluster/Nodes/NodeDetails.vue'
import NodeExtensions from '@/views/cluster/Nodes/NodeExtensions.vue'
import NodeLogs from '@/views/cluster/Nodes/NodeLogs.vue'
import NodeMonitor from '@/views/cluster/Nodes/NodeMonitor.vue'
import NodeMounts from '@/views/cluster/Nodes/NodeMounts.vue'
import NodeOverview from '@/views/cluster/Nodes/NodeOverview.vue'
import NodePatches from '@/views/cluster/Nodes/NodePatches.vue'
import NodesList from '@/views/cluster/Nodes/NodesList.vue'
import ClusterOverview from '@/views/cluster/Overview/Overview.vue'
import TPods from '@/views/cluster/Pods/TPods.vue'
import ClusterSidebar from '@/views/cluster/SideBar.vue'
import ClusterSidebarNode from '@/views/cluster/SideBarNode.vue'
import BadRequest from '@/views/common/BadRequest.vue'
import Forbidden from '@/views/common/Forbidden.vue'
import PageNotFound from '@/views/common/PageNotFound.vue'
import Authenticate from '@/views/omni/Auth/Authenticate.vue'
import OIDC from '@/views/omni/Auth/OIDC.vue'
import OmniClusters from '@/views/omni/Clusters/Clusters.vue'
import OmniClusterCreate from '@/views/omni/Clusters/Management/ClusterCreate.vue'
import OmniClusterScale from '@/views/omni/Clusters/Management/ClusterScale.vue'
import Home from '@/views/omni/Home/Home.vue'
import OmniMachineClass from '@/views/omni/MachineClasses/MachineClass.vue'
import OmniMachineClasses from '@/views/omni/MachineClasses/MachineClasses.vue'
import OmniMachine from '@/views/omni/Machines/Machine.vue'
import OmniMachineLogs from '@/views/omni/Machines/MachineLogs.vue'
import OmniMachinePatches from '@/views/omni/Machines/MachinePatches.vue'
import OmniMachines from '@/views/omni/Machines/Machines.vue'
import OmniMachinesPending from '@/views/omni/Machines/MachinesPending.vue'
import ClusterDestroy from '@/views/omni/Modals/ClusterDestroy.vue'
import ConfigPatchDestroy from '@/views/omni/Modals/ConfigPatchDestroy.vue'
import DownloadInstallationMedia from '@/views/omni/Modals/DownloadInstallationMedia.vue'
import DownloadOmnictl from '@/views/omni/Modals/DownloadOmnictl.vue'
import DownloadSupportBundle from '@/views/omni/Modals/DownloadSupportBundle.vue'
import DownloadTalosctl from '@/views/omni/Modals/DownloadTalosctl.vue'
import InfraProviderDelete from '@/views/omni/Modals/InfraProviderDelete.vue'
import InfraProviderSetup from '@/views/omni/Modals/InfraProviderSetup.vue'
import JoinTokenCreate from '@/views/omni/Modals/JoinTokenCreate.vue'
import JoinTokenDelete from '@/views/omni/Modals/JoinTokenDelete.vue'
import JoinTokenRevoke from '@/views/omni/Modals/JoinTokenRevoke.vue'
import MachineAccept from '@/views/omni/Modals/MachineAccept.vue'
import MachineClassDestroy from '@/views/omni/Modals/MachineClassDestroy.vue'
import MachineReject from '@/views/omni/Modals/MachineReject.vue'
import MachineRemove from '@/views/omni/Modals/MachineRemove.vue'
import MachineSetDestroy from '@/views/omni/Modals/MachineSetDestroy.vue'
import MaintenanceUpdate from '@/views/omni/Modals/MaintenanceUpdate.vue'
import NodeDestroy from '@/views/omni/Modals/NodeDestroy.vue'
import NodeDestroyCancel from '@/views/omni/Modals/NodeDestroyCancel.vue'
import NodeReboot from '@/views/omni/Modals/NodeReboot.vue'
import NodeShutdown from '@/views/omni/Modals/NodeShutdown.vue'
import RoleEdit from '@/views/omni/Modals/RoleEdit.vue'
import ServiceAccountCreate from '@/views/omni/Modals/ServiceAccountCreate.vue'
import ServiceAccountRenew from '@/views/omni/Modals/ServiceAccountRenew.vue'
import UpdateExtensions from '@/views/omni/Modals/UpdateExtensions.vue'
import UpdateKubernetes from '@/views/omni/Modals/UpdateKubernetes.vue'
import UpdateTalos from '@/views/omni/Modals/UpdateTalos.vue'
import UserCreate from '@/views/omni/Modals/UserCreate.vue'
import UserDestroy from '@/views/omni/Modals/UserDestroy.vue'
import OmniBackupStorageSettings from '@/views/omni/Settings/BackupStorage.vue'
import OmniInfraProviders from '@/views/omni/Settings/InfraProviders.vue'
import OmniJoinTokens from '@/views/omni/Settings/JoinTokens.vue'
import OmniSettings from '@/views/omni/Settings/Settings.vue'
import OmniSidebar from '@/views/omni/SideBar.vue'
import OmniServiceAccounts from '@/views/omni/Users/ServiceAccounts.vue'
import OmniUsers from '@/views/omni/Users/Users.vue'

export const FrontendAuthFlow = 'frontend'
const requireCookies = false

const routes: RouteRecordRaw[] = [
  // Unauthenticated routes
  { path: '/forbidden', component: Forbidden },
  { path: '/badrequest', component: BadRequest },
  { path: '/:catchAll(.*)', component: PageNotFound },
  {
    path: '/authenticate',
    name: 'Authenticate',
    component: Authenticate,
    beforeEnter: async (to) => {
      return authType.value === AuthType.Auth0 ? await authGuard(to) : true
    },
  },

  // Redirects for legacy routes
  { path: '/omni', redirect: '/' },
  { path: '/omni/:catchAll(.*)', redirect: (to) => `/${to.params.catchAll}` },
  { path: '/cluster', redirect: '/clusters' },
  { path: '/cluster/:catchAll(.*)', redirect: (to) => `/clusters/${to.params.catchAll}` },

  // Authenticated routes
  {
    path: '/',
    components: {
      default: RouterView,
      sidebar: OmniSidebar,
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

      return { name: 'Authenticate', query: { flow: FrontendAuthFlow, redirect: to.fullPath } }
    },
    children: [
      {
        path: '',
        name: 'Home',
        component: Home,
      },
      {
        path: 'oidc-login/:authRequestId',
        name: 'OIDC Login',
        component: OIDC,
      },
      {
        path: 'clusters',
        children: [
          {
            path: '',
            name: 'Clusters',
            component: OmniClusters,
          },
          {
            path: 'create',
            name: 'ClusterCreate',
            component: OmniClusterCreate,
          },
          {
            path: ':cluster',
            components: {
              default: ClusterScoped,
              clusterSidebar: ClusterSidebar,
            },
            children: [
              {
                path: '',
                name: 'ClusterOverview',
                component: ClusterOverview,
              },
              {
                path: 'nodes',
                name: 'Nodes',
                component: NodesList,
              },
              {
                path: 'scale',
                name: 'ClusterScale',
                component: OmniClusterScale,
              },
              {
                path: 'pods',
                name: 'Pods',
                component: TPods,
              },
              {
                path: 'patches',
                name: 'ClusterConfigPatches',
                component: ClusterPatches,
              },
              {
                path: 'patches/:patch',
                name: 'ClusterPatchEdit',
                component: PatchEdit,
              },
              {
                path: 'manifests',
                name: 'KubernetesManifestSync',
                component: KubernetesManifestSync,
              },
              {
                path: 'backups',
                name: 'Backups',
                component: ClusterBackups,
              },
              {
                path: 'machine/:machine',
                components: {
                  default: RouterView,
                  nodeSidebar: ClusterSidebarNode,
                },
                children: [
                  {
                    path: 'patches/:patch',
                    name: 'ClusterMachinePatchEdit',
                    component: PatchEdit,
                  },
                  {
                    path: '',
                    name: 'NodeDetails',
                    component: NodeDetails,
                    children: [
                      {
                        path: '',
                        name: 'NodeOverview',
                        component: NodeOverview,
                      },
                      {
                        path: 'monitor',
                        name: 'NodeMonitor',
                        component: NodeMonitor,
                      },
                      {
                        path: 'logs/:service',
                        name: 'NodeLogs',
                        component: NodeLogs,
                      },
                      {
                        path: 'config',
                        name: 'NodeConfig',
                        component: NodeConfig,
                      },
                      {
                        path: 'patches',
                        name: 'NodePatches',
                        component: NodePatches,
                      },
                      {
                        path: 'mounts',
                        name: 'NodeMounts',
                        component: NodeMounts,
                      },
                      {
                        path: 'extensions',
                        name: 'NodeExtensions',
                        component: NodeExtensions,
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
        name: 'Machines',
        component: OmniMachines,
      },
      {
        path: 'machines/manual',
        name: 'MachinesManual',
        component: OmniMachines,
        props: {
          filter: MachineFilterOption.Manual,
        },
      },
      {
        path: 'machines/managed',
        name: 'MachinesManaged',
        component: OmniMachines,
        props: {
          filter: MachineFilterOption.Managed,
        },
      },
      {
        path: 'machines/managed/:provider',
        name: 'MachinesManagedProvider',
        component: OmniMachines,
      },
      {
        path: 'machines/pending',
        name: 'MachinesPending',
        component: OmniMachinesPending,
      },
      {
        path: 'machine-classes',
        name: 'MachineClasses',
        component: OmniMachineClasses,
      },
      {
        path: 'machine-classes/create',
        name: 'MachineClassCreate',
        component: OmniMachineClass,
      },
      {
        path: 'machine-classes/:classname',
        name: 'MachineClassEdit',
        component: OmniMachineClass,
        props: {
          edit: true,
        },
      },
      {
        path: 'machine/:machine/patches/:patch',
        name: 'MachinePatchEdit',
        component: PatchEdit,
      },
      {
        path: 'machine/jointokens',
        name: 'JoinTokens',
        component: OmniJoinTokens,
      },
      {
        path: 'settings',
        name: 'Settings',
        component: OmniSettings,
        redirect: {
          name: 'Users',
        },
        children: [
          {
            path: 'users',
            name: 'Users',
            component: OmniUsers,
            meta: {
              title: 'Users',
            },
          },
          {
            path: 'serviceaccounts',
            name: 'ServiceAccounts',
            component: OmniServiceAccounts,
            meta: {
              title: 'Service Accounts',
            },
          },
          {
            path: 'infraproviders',
            name: 'InfraProviders',
            component: OmniInfraProviders,
            meta: {
              title: 'Infra Providers',
            },
          },
          {
            path: 'backups',
            name: 'BackupStorage',
            component: OmniBackupStorageSettings,
            meta: {
              title: 'Backup Storage',
            },
          },
        ],
      },
      {
        path: 'machine/:machine',
        name: 'Machine',
        component: OmniMachine,
        redirect: {
          name: 'MachineLogs',
        },
        children: [
          {
            path: 'logs',
            name: 'MachineLogs',
            component: OmniMachineLogs,
          },
          {
            path: 'patches',
            name: 'MachineConfigPatches',
            component: OmniMachinePatches,
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

  current.value = to.params.cluster

  return true
})

router.afterEach(() => {
  Userpilot.reload()
})

const modals = {
  reboot: NodeReboot,
  shutdown: NodeShutdown,
  clusterDestroy: ClusterDestroy,
  machineRemove: MachineRemove,
  machineClassDestroy: MachineClassDestroy,
  machineSetDestroy: MachineSetDestroy,
  maintenanceUpdate: MaintenanceUpdate,
  downloadInstallationMedia: DownloadInstallationMedia,
  downloadOmnictlBinaries: DownloadOmnictl,
  downloadSupportBundle: DownloadSupportBundle,
  downloadTalosctlBinaries: DownloadTalosctl,
  nodeDestroy: NodeDestroy,
  nodeDestroyCancel: NodeDestroyCancel,
  updateKubernetes: UpdateKubernetes,
  updateTalos: UpdateTalos,
  configPatchDestroy: ConfigPatchDestroy,
  userDestroy: UserDestroy,
  userCreate: UserCreate,
  joinTokenCreate: JoinTokenCreate,
  joinTokenRevoke: JoinTokenRevoke,
  joinTokenDelete: JoinTokenDelete,
  serviceAccountCreate: ServiceAccountCreate,
  serviceAccountRenew: ServiceAccountRenew,
  roleEdit: RoleEdit,
  machineAccept: MachineAccept,
  machineReject: MachineReject,
  updateExtensions: UpdateExtensions,
  infraProviderSetup: InfraProviderSetup,
  infraProviderDelete: InfraProviderDelete,
}

export { modals }
export default router
