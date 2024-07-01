// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

import { createWebHistory, createRouter, RouteLocation, RouteRecordRaw, RouteLocationRaw } from "vue-router";

// Root level routes
import OmniOverview from "@/views/omni/Overview/Overview.vue";
import OmniClusters from "@/views/omni/Clusters/Clusters.vue";
import OmniClusterCreate from "@/views/omni/Clusters/Management/ClusterCreate.vue";
import OmniClusterScale from "@/views/omni/Clusters/Management/ClusterScale.vue";
import OmniMachines from "@/views/omni/Machines/Machines.vue";
import OmniMachineLogs from "@/views/omni/Machines/MachineLogs.vue";
import OmniMachinePatches from "@/views/omni/Machines/MachinePatches.vue";
import OmniMachine from "@/views/omni/Machines/Machine.vue";
import OmniUsers from "@/views/omni/Users/Users.vue";
import OmniSettings from "@/views/omni/Settings/Settings.vue";
import Authenticate from "@/views/omni/Auth/Authenticate.vue";
import OmniMachineClasses from "@/views/omni/MachineClasses/MachineClasses.vue";
import OmniMachineClass from "@/views/omni/MachineClasses/MachineClass.vue";
import OmniBackupStorageSettings from "@/views/omni/Settings/BackupStorage.vue";
import OIDC from "@/views/omni/Auth/OIDC.vue";

// Cluster level routes
import ClusterScoped from "@/views/cluster/ClusterScoped.vue";
import NodesList from "@/views/cluster/Nodes/NodesList.vue";
import TPods from "@/views/cluster/Pods/TPods.vue";
import ClusterOverview from "@/views/cluster/Overview/Overview.vue";
import NodeOverview from "@/views/cluster/Nodes/NodeOverview.vue";
import NodeMonitor from "@/views/cluster/Nodes/NodeMonitor.vue";
import NodeLogs from "@/views/cluster/Nodes/NodeLogs.vue";
import NodeConfig from "@/views/cluster/Nodes/NodeConfig.vue";
import NodeMounts from "@/views/cluster/Nodes/NodeMounts.vue";
import NodeExtensions from "@/views/cluster/Nodes/NodeExtensions.vue";
import NodePatches from "@/views/cluster/Nodes/NodePatches.vue";
import ClusterPatches from "@/views/cluster/Config/ClusterPatches.vue";
import PatchEdit from "@/views/cluster/Config/PatchEdit.vue";
import KubernetesManifestSync from "@/views/cluster/Manifest/Sync.vue";
import NodeDetails from "@/views/cluster/Nodes/NodeDetails.vue";
import ClusterBackups from "@/views/cluster/Backups/Backups.vue";

import PageNotFound from "@/views/common/PageNotFound.vue";
import Forbidden from "@/views/common/Forbidden.vue";

// sidebars
import OmniSidebar from "@/views/omni/SideBar.vue";
import ClusterSidebar from "@/views/cluster/SideBar.vue";
import ClusterSidebarNode from "@/views/cluster/SideBarNode.vue";

// modal windows
import NodeReboot from "@/views/omni/Modals/NodeReboot.vue";
import NodeShutdown from "@/views/omni/Modals/NodeShutdown.vue";
import ClusterDestroy from "@/views/omni/Modals/ClusterDestroy.vue";
import DownloadSupportBundle from "@/views/omni/Modals/DownloadSupportBundle.vue";
import MachineSetDestroy from "@/views/omni/Modals/MachineSetDestroy.vue";
import ConfigPatchDestroy from "@/views/omni/Modals/ConfigPatchDestroy.vue";
import UpdateExtensions from "@/views/omni/Modals/UpdateExtensions.vue"
import NodeDestroy from "@/views/omni/Modals/NodeDestroy.vue";
import MachineRemove from "@/views/omni/Modals/MachineRemove.vue";
import MachineClassDestroy from "@/views/omni/Modals/MachineClassDestroy.vue";
import MaintenanceUpdate from "@/views/omni/Modals/MaintenanceUpdate.vue";
import NodeDestroyCancel from "@/views/omni/Modals/NodeDestroyCancel.vue";
import DownloadInstallationMedia from "@/views/omni/Modals/DownloadInstallationMedia.vue";
import DownloadOmnictl from "@/views/omni/Modals/DownloadOmnictl.vue";
import DownloadTalosctl from "@/views/omni/Modals/DownloadTalosctl.vue";
import UserDestroy from "@/views/omni/Modals/UserDestroy.vue";
import UpdateKubernetes from "@/views/omni/Modals/UpdateKubernetes.vue";
import UpdateTalos from "@/views/omni/Modals/UpdateTalos.vue";
import UserCreate from "@/views/omni/Modals/UserCreate.vue";
import UserEdit from "@/views/omni/Modals/UserEdit.vue";

import { current } from "@/context";
import { authGuard } from "@auth0/auth0-vue";
import { AuthType, authType } from "@/methods";
import { getAuthCookies, isAuthorized } from "@/methods/key";
import { refreshTitle } from "@/methods/title";
import { loadCurrentUser } from "@/methods/auth";

export const FrontendAuthFlow = "frontend";

const withPrefix = (prefix: string, routes: RouteRecordRaw[], meta?: Record<string, any>) =>
  routes.map((route) => {
    if (meta && !route.meta) {
      route.meta = meta;
    }

    if (!route.beforeEnter) {
      route.beforeEnter = (to: RouteLocation) => {
        return checkAuthorized(to)
      }
    }

    route.path = prefix + route.path;
    return route;
  }
);

export const checkAuthorized =  async (to: RouteLocation, requireCookies?: boolean): Promise<RouteLocationRaw | boolean> => {
  let authorized = await isAuthorized();

  if (requireCookies && !getAuthCookies()) {
    authorized = false;
  }

  if (authorized) {
    await loadCurrentUser()
  }

  if (authorized) {
    await refreshTitle()

    return true;
  }

  return { name: "Authenticate", query: { flow: FrontendAuthFlow, redirect: to.fullPath } };
}

export function getBreadcrumbs(route: RouteLocation) {
  const crumbs: {text: string, to?: { name: string, query: any}}[] = [];

  if (route.params.machine) {
    if (route.params.cluster) {
      crumbs.push(
        {
          text: `All Nodes`,
          to: { name: "Nodes", query: route.query },
        },
        {
          text: `${route.params.machine}`,
          to: { name: "NodeOverview", query: route.query },
        }
      );
    } else {
      crumbs.push(
        {
          text: "Machines",
          to: { name: "Machines", query: route.query },
        },
      )
    }
  }

  if (route.params.service) {
    crumbs.push({
      text: `${route.params.service} Logs`,
      to: { name: "NodeLogs", query: route.query },
    });
  }

  return crumbs;
}

export function getSidebar(route) {
  if (route.meta.sidebar) {
    return route.meta.sidebar
  }

  return null
}

const beforeEnter = async (to: RouteLocation) => {
  if (authType.value === AuthType.Auth0) {
    return await authGuard(to);
  }

  return true;
}

const routes: RouteRecordRaw[] = [
  { path: '/', redirect: '/omni/' },
  { path: "/forbidden", component: Forbidden },
  ...withPrefix("/omni", [
    {
      path: "/authenticate",
      name: "Authenticate",
      component: Authenticate,
      beforeEnter: beforeEnter,
    },
    {
      path: "/oidc-login/:authRequestId",
      name: "OIDC Login",
      component: OIDC,
    },
  ]),
  ...withPrefix("/omni", [
    {
      path: "/",
      name: "Overview",
      component: OmniOverview,
    },
    {
      path: "/clusters",
      name: "Clusters",
      component:  OmniClusters,
    },
    {
      path: "/cluster/create",
      name: "ClusterCreate",
      component: OmniClusterCreate,
    },
    {
      path: "/machines",
      name: "Machines",
      component:  OmniMachines,
    },
    {
      path: "/machine-classes",
      name: "MachineClasses",
      component: OmniMachineClasses,
    },
    {
      path: "/machine-classes/create",
      name: "MachineClassCreate",
      component: OmniMachineClass,
    },
    {
      path: "/machine-classes/:classname",
      name: "MachineClassEdit",
      component: OmniMachineClass,
      props: {
        edit: true,
      }
    },
    {
      path: "/machine/:machine/patches/:patch",
      name: "MachinePatchEdit",
      component:  PatchEdit,
    },
    {
      path: "/settings",
      name: "Settings",
      component: OmniSettings,
      redirect: {
        name: "Users"
      },
      children: [
        {
          path: "users",
          name: "Users",
          components: {
            inner: OmniUsers,
          }
        },
        {
          path: "backups",
          name: "BackupStorage",
          components: {
            inner: OmniBackupStorageSettings,
          }
        },
      ]
    },
    {
      path: "/machine/:machine",
      name: "Machine",
      component: OmniMachine,
      redirect: {
        name: "MachineLogs",
      },
      children: [
        {
          path: "logs",
          name: "MachineLogs",
          components: {
            inner: OmniMachineLogs,
          }
        },
        {
          path: "patches",
          name: "MachineConfigPatches",
          components: {
            inner: OmniMachinePatches,
          }
        },
      ]
    }
  ], { sidebar: OmniSidebar }),
  ...withPrefix("/cluster/:cluster", [
    {
      path: "/nodes",
      name: "Nodes",
      component: ClusterScoped,
      props: {
        inner: NodesList,
      }
    },
    {
      path: "/scale",
      name: "ClusterScale",
      component: ClusterScoped,
      props: {
        inner: OmniClusterScale,
      },
    },
    {
      path: "/pods",
      name: "Pods",
      component: ClusterScoped,
      props: {
        inner: TPods,
      },
    },
    {
      path: "/overview",
      name: "ClusterOverview",
      component: ClusterScoped,
      props: {
        inner: ClusterOverview,
      },
    },
    {
      path: "/patches",
      name: "ClusterConfigPatches",
      component: ClusterScoped,
      props: {
        inner: ClusterPatches,
      },
    },
    {
      path: "/patches/:patch",
      name: "ClusterPatchEdit",
      component: ClusterScoped,
      props: {
        inner: PatchEdit,
      },
    },
    {
      path: "/manifests",
      name: "KubernetesManifestSync",
      component: ClusterScoped,
      props: {
        inner: KubernetesManifestSync,
      },
    },
    {
      path: "/backups",
      name: "Backups",
      component: ClusterBackups,
    },
    ...withPrefix("/machine/:machine", [
      {
        path: "/patches/:patch",
        name: "ClusterMachinePatchEdit",
        component: ClusterScoped,
        props: {
          inner: PatchEdit,
        }
      },
      {
        path: "/",
        name: "NodeDetails",
        component: ClusterScoped,
        props: {
          inner: NodeDetails,
        },
        children: [
          {
            path: "overview",
            name: "NodeOverview",
            components: {
              nodeDetails: NodeOverview,
            },
          },
          {
            path: "monitor",
            name: "NodeMonitor",
            components: {
              nodeDetails: NodeMonitor,
            },
          },
          {
            path: "logs/:service",
            name: "NodeLogs",
            components: {
              nodeDetails: NodeLogs,
            },
          },
          {
            path: "config",
            name: "NodeConfig",
            components: {
              nodeDetails: NodeConfig,
            },
          },
          {
            path: "patches",
            name: "NodePatches",
            components: {
              nodeDetails: NodePatches,
            },
          },
          {
            path: "mounts",
            name: "NodeMounts",
            components: {
              nodeDetails: NodeMounts,
            },
          },
          {
            path: "extensions",
            name: "NodeExtensions",
            components: {
              nodeDetails: NodeExtensions,
            },
          },
        ]
      },
    ], { sidebar: ClusterSidebarNode }),
  ], { sidebar: ClusterSidebar }),
  { path: "/:catchAll(.*)", component: PageNotFound },
];

const router = createRouter({
  history: createWebHistory(),
  routes,
});

router.beforeEach(to => {
  if (!to.params.cluster) {
    return true;
  }

  current.value = to.params.cluster;

  return true;
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
  userEdit: UserEdit,
  updateExtensions: UpdateExtensions,
};

export { modals };
export default router;
