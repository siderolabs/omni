// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import type { Meta, StoryObj } from '@storybook/vue3-vite'

import type { Resource } from '@/api/grpc'
import type { ClusterSpec, KubernetesHealthCheckStatusSpec } from '@/api/omni/specs/omni.pb'
import {
  KubernetesHealthCheckStatusSpecState,
  KubernetesUpgradeStatusSpecPhase,
  SecretRotationSpecComponent,
  SecretRotationSpecPhase,
  TalosUpgradeStatusSpecPhase,
} from '@/api/omni/specs/omni.pb'
import { ClusterLocked, LabelCluster } from '@/api/resources'

import { CLUSTER_ID, createHandlers, defaultMachines, TALOS_VERSION } from './OverviewContent.mocks'
import OverviewContent from './OverviewContent.vue'

const currentCluster: Resource<ClusterSpec> = {
  metadata: { id: CLUSTER_ID },
  spec: { talos_version: TALOS_VERSION, kubernetes_version: '1.31.2' },
}

const meta: Meta<typeof OverviewContent> = {
  component: OverviewContent,
  args: {
    currentCluster,
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {
  beforeEach({ msw }) {
    msw.use(...createHandlers())
  },
}

export const KubernetesUpgrading: Story = {
  beforeEach({ msw }) {
    msw.use(
      ...createHandlers({
        kubernetesUpgradeStatus: {
          phase: KubernetesUpgradeStatusSpecPhase.Upgrading,
          step: 'updating kubelet',
          status: 'talos-default-cp-1',
          last_upgrade_version: '1.30.4',
          current_upgrade_version: '1.31.2',
        },
      }),
    )
  },
}

export const KubernetesReverting: Story = {
  beforeEach({ msw }) {
    msw.use(
      ...createHandlers({
        kubernetesUpgradeStatus: {
          phase: KubernetesUpgradeStatusSpecPhase.Reverting,
          step: 'reverting kubelet',
          status: 'talos-default-cp-1',
          last_upgrade_version: '1.30.4',
        },
      }),
    )
  },
}

export const TalosUpgrading: Story = {
  beforeEach({ msw }) {
    msw.use(
      ...createHandlers({
        talosUpgradeStatus: {
          phase: TalosUpgradeStatusSpecPhase.Upgrading,
          status: 'updating machines 1/3',
          step: 'current machine: talos-default-cp-1',
          last_upgrade_version: 'v1.8.4',
          current_upgrade_version: TALOS_VERSION,
        },
      }),
    )
  },
}

export const KubernetesUpgradeMachineLocked: Story = {
  beforeEach({ msw }) {
    msw.use(
      ...createHandlers({
        kubernetesUpgradeStatus: {
          phase: KubernetesUpgradeStatusSpecPhase.Upgrading,
          step: 'updating kubelet to 1.31.2',
          // the backend formats the blocked node names with Go's %q on a []string
          status: `waiting for a machine from ["${CLUSTER_ID}-worker-1" "${CLUSTER_ID}-worker-2"] to be unlocked`,
          last_upgrade_version: '1.30.4',
          current_upgrade_version: '1.31.2',
        },
      }),
    )
  },
}

export const TalosUpgradeMachinesLocked: Story = {
  beforeEach({ msw }) {
    msw.use(
      ...createHandlers({
        talosUpgradeStatus: {
          phase: TalosUpgradeStatusSpecPhase.Upgrading,
          status: 'waiting for machines to be unlocked',
          step: 'all machines are locked',
          last_upgrade_version: 'v1.8.4',
          current_upgrade_version: TALOS_VERSION,
        },
      }),
    )
  },
}

export const SecretRotationMachinesLocked: Story = {
  beforeEach({ msw }) {
    msw.use(
      ...createHandlers({
        secretRotationStatus: {
          phase: SecretRotationSpecPhase.ROTATE,
          component: SecretRotationSpecComponent.TALOS_CA,
          status: 'rotation paused at phase ROTATE',
          step: `waiting for machines: [${CLUSTER_ID}-worker-1]`,
        },
      }),
    )
  },
}

export const ClusterHealthChecks: Story = {
  beforeEach({ msw }) {
    const kubeProxyError = 'daemonset "kube-proxy" is not ready: 2/3 pods ready'

    const healthChecks: Resource<KubernetesHealthCheckStatusSpec>[] = [
      {
        spec: {
          state: KubernetesHealthCheckStatusSpecState.PASSED,
          job_namespace: 'kube-system',
          job_name: 'omni-healthcheck-apiserver-ready',
        },
        metadata: { id: 'apiserver-ready', labels: { [LabelCluster]: CLUSTER_ID } },
      },
      {
        // a previous failure stays readable while the next attempt runs
        spec: {
          state: KubernetesHealthCheckStatusSpecState.RUNNING,
          job_namespace: 'kube-system',
          job_name: 'omni-healthcheck-coredns-ready',
          error: 'deployment "coredns" is not ready: 1/2 replicas available',
          output: 'checking deployment "coredns"\n1/2 replicas available\ntimed out after 5m0s',
        },
        metadata: { id: 'coredns-ready', labels: { [LabelCluster]: CLUSTER_ID } },
      },
      {
        spec: {
          state: KubernetesHealthCheckStatusSpecState.FAILED,
          job_namespace: 'kube-system',
          job_name: 'omni-healthcheck-kube-proxy-ready',
          error: kubeProxyError,
          output: `checking daemonset "kube-proxy"\n${kubeProxyError}\ntimed out after 5m0s`,
        },
        metadata: { id: 'kube-proxy-ready', labels: { [LabelCluster]: CLUSTER_ID } },
      },
    ]

    msw.use(
      ...createHandlers({
        talosUpgradeStatus: {
          phase: TalosUpgradeStatusSpecPhase.Upgrading,
          status: 'updating machines 2/3',
          step: `waiting for healthchecks to pass: "coredns-ready"; "kube-proxy-ready": ${kubeProxyError}`,
          last_upgrade_version: 'v1.8.4',
          current_upgrade_version: TALOS_VERSION,
        },
        healthChecks,
      }),
    )
  },
}

export const ClusterIsLocked: Story = {
  args: {
    currentCluster: {
      ...currentCluster,
      metadata: { ...currentCluster.metadata, annotations: { [ClusterLocked]: '' } },
    },
  },
  beforeEach({ msw }) {
    msw.use(
      ...createHandlers({
        talosUpgradeStatus: {
          phase: TalosUpgradeStatusSpecPhase.Upgrading,
          // machine updates are held while the cluster is locked, so the rollout stays on the
          // machine it was on when the lock was applied
          status: 'updating machines 1/3',
          step: 'current machine: talos-default-cp-1',
          last_upgrade_version: 'v1.8.4',
          current_upgrade_version: TALOS_VERSION,
        },
      }),
    )
  },
}

export const MachineWithConfigError: Story = {
  beforeEach({ msw }) {
    const [controlPlaneMachine, ...rest] = defaultMachines

    msw.use(
      ...createHandlers({
        machines: [
          {
            ...controlPlaneMachine,
            spec: {
              ...controlPlaneMachine.spec,
              last_config_error: 'failed to apply config: rpc error: context deadline exceeded',
            },
          },
          ...rest,
        ],
      }),
    )
  },
}
