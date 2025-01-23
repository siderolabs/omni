// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

import { Resource } from "../../src/api/grpc";
import { ClusterSpec, ConfigPatchSpec, MachineSetNodeSpec, MachineSetSpec, MachineSetSpecMachineAllocationSource, MachineSetSpecMachineAllocationType, MachineSetSpecUpdateStrategy } from "../../src/api/omni/specs/omni.pb";
import { ClusterType, ConfigPatchType, DefaultNamespace, LabelCluster, LabelClusterMachine, LabelControlPlaneRole, LabelMachineSet, LabelWorkerRole, MachineSetNodeType, MachineSetType } from "../../src/api/resources";
import { Cluster, initState, MachineSet, PatchID, state } from "../../src/states/cluster-management";

import { describe, expect, test } from "bun:test";

const crypto = require('crypto');

Object.defineProperty(globalThis, 'crypto', {
  value: {
    getRandomValues: arr => crypto.randomBytes(arr.length)
  }
});

describe("cluster-management-state", () => {
  initState();

  const tests: {
    name: string
    cluster: Cluster
    machineSets?: Partial<MachineSet>[]
    shouldFail?: boolean
    base?: Resource[]
    expectedResources?: {
      [ConfigPatchType]?: Record<string, Partial<Resource<ConfigPatchSpec>>>
      [MachineSetType]?: Record<string, Partial<Resource<MachineSetSpec>>>
      [ClusterType]?: Record<string, Partial<Resource<ClusterSpec>>>
      [MachineSetNodeType]?: Record<string, Partial<Resource<MachineSetNodeSpec>>>
    }
  }[] = [
    {
      name: "no cluster name",
      cluster: {
        patches: {},
        features: {},
      },
      shouldFail: true,
    },
    {
      name: "everything set",
      cluster: {
        patches: {
          [PatchID.Default]: {
            weight: 400,
            data: "abcd"
          },
          additional: {
            weight: 200,
            data: "aaaa"
          }
        },
        features: {
          enableWorkloadProxy: true,
          useEmbeddedDiscoveryService: true,
          encryptDisks: true,
        },
        labels: {
          my: "label",
        },
        annotations: {
          my: "annotation",
        },
        kubernetesVersion: "1.24.0",
        talosVersion: "1.5.3",
        etcdBackupConfig: {
          enabled: true,
          interval: "100s",
        },
        name: "talos-default",
      },
      machineSets: [
        {
          machineAllocation: {
            name: "mc1",
            size: "unlimited",
            source: MachineSetSpecMachineAllocationSource.MachineClass,
          },
          machines: {
            node4: {
              patches: {},
            },
          },
          bootstrapSpec: {
            cluster_uuid: "abcd",
            snapshot: "dcba",
          }
        },
        {
          machineAllocation: {
            name: "mc2",
            size: 3,
            source: MachineSetSpecMachineAllocationSource.MachineClass,
          },
          patches: {
            [PatchID.Default]: {
              data: "works",
              weight: 200,
            },
            [PatchID.Untaint]: {
              data: "untaint",
              weight: 1,
            }
          }
        },
        {
          machines: {
            node1: {
              patches: {
                "cm-node1": {
                  weight: 100,
                  data: "works",
                }
              },
            },
          }
        },
      ],
      expectedResources: {
        [ClusterType]: {
          "talos-default": {
            metadata: {
              labels: {
                my: "label",
              },
              annotations: {
                my: "annotation",
              },
            },
            spec: {
              features: {
                disk_encryption: true,
                enable_workload_proxy: true,
                use_embedded_discovery_service: true,
              },
              kubernetes_version: "1.24.0",
              talos_version: "1.5.3",
              backup_configuration: {
                enabled: true,
                interval: "100s",
              }
            }
          }
        },
        [MachineSetNodeType]: {
          "node1": {
            metadata: {
              labels: {
                [LabelCluster]: "talos-default",
                [LabelMachineSet]: "talos-default-w000000",
                [LabelWorkerRole]: ""
              },
            },
            spec: {},
          }
        },
        [MachineSetType]: {
          "talos-default-control-planes": {
            metadata: {
              labels: {
                [LabelCluster]: "talos-default",
                [LabelControlPlaneRole]: ""
              }
            },
            spec: {
              bootstrap_spec: {
                cluster_uuid: "abcd",
                snapshot: "dcba",
              },
              update_strategy: MachineSetSpecUpdateStrategy.Rolling,
              machine_allocation: {
                name: "mc1",
                allocation_type: MachineSetSpecMachineAllocationType.Unlimited,
                source: MachineSetSpecMachineAllocationSource.MachineClass,
              },
              machine_class: undefined,
            }
          },
          "talos-default-workers": {
            metadata: {
              labels: {
                [LabelCluster]: "talos-default",
                [LabelWorkerRole]: ""
              }
            },
            spec: {
              bootstrap_spec: undefined,
              update_strategy: MachineSetSpecUpdateStrategy.Rolling,
              machine_allocation: {
                name: "mc2",
                machine_count: 3,
                source: MachineSetSpecMachineAllocationSource.MachineClass,
              },
              machine_class: undefined
            }
          },
          "talos-default-w000000": {
            metadata: {
              labels: {
                [LabelCluster]: "talos-default",
                [LabelWorkerRole]: ""
              }
            },
            spec: {
              bootstrap_spec: undefined,
              update_strategy: MachineSetSpecUpdateStrategy.Rolling,
              machine_class: undefined,
            }
          }
        },
        [ConfigPatchType]: {
          "400-talos-default": {
            metadata: {
              labels: {
                [LabelCluster]: "talos-default",
              }
            },
            spec: {
              data: "abcd"
            },
          },
          "200-talos-default-additional": {
            metadata: {
              labels: {
                [LabelCluster]: "talos-default",
              }
            },
            spec: {
              data: "aaaa"
            },
          },
          "200-talos-default-workers": {
            metadata: {
              labels: {
                [LabelCluster]: "talos-default",
                [LabelMachineSet]: "talos-default-workers"
              }
            },
            spec: {
              data: "works"
            },
          },
          "001-talos-default-workers-untaint": {
            metadata: {
              labels: {
                [LabelCluster]: "talos-default",
                [LabelMachineSet]: "talos-default-workers"
              }
            },
            spec: {
              data: "untaint"
            },
          },
          "100-cm-node1": {
            metadata: {
              labels: {
                [LabelCluster]: "talos-default",
                [LabelClusterMachine]: "node1"
              }
            },
            spec: {
              data: "works"
            },
          }
        }
      }
    },
    {
      name: "update base",
      cluster: {
        name: "talos-default",
        kubernetesVersion: "1.29.0",
        talosVersion: "1.4.5",
        patches: {},
        features: {},
      },
      base: [
        {
          metadata: {
            type: ClusterType,
            id: "talos-default",
            labels: {
              my: "label",
            },
            annotations: {
              my: "annotation",
            },
          },
          spec: {
            features: {
              disk_encryption: true,
              enable_workload_proxy: true,
            },
            kubernetes_version: "1.24.0",
            talos_version: "1.5.3",
            backup_configuration: {
              enabled: true,
              interval: "100s",
            }
          }
        },
      ],
      expectedResources: {
        [ClusterType]: {
          "talos-default": {
            metadata: {
              annotations: undefined,
              labels: undefined,
            },
            spec: {
              kubernetes_version: "1.29.0",
              talos_version: "1.4.5",
              backup_configuration: undefined,
              features: {},
            }
          }
        },
        [MachineSetType]: {
          "talos-default-control-planes": {
            metadata: {
              labels: {
                [LabelCluster]: "talos-default",
                [LabelControlPlaneRole]: ""
              }
            },
            spec: {
              bootstrap_spec: undefined,
              update_strategy: MachineSetSpecUpdateStrategy.Rolling,
              machine_class: undefined,
              machine_allocation: undefined,
            }
          },
          "talos-default-workers": {
            metadata: {
              labels: {
                [LabelCluster]: "talos-default",
                [LabelWorkerRole]: ""
              }
            },
            spec: {
              bootstrap_spec: undefined,
              update_strategy: MachineSetSpecUpdateStrategy.Rolling,
              machine_class: undefined,
              machine_allocation: undefined,
            }
          },
        },
      },
    },
  ];

  for (const tt of tests) {
    test(`resource generation: ${tt.name}`, () => {
      const run = () => {
        initState()

        state.value.cluster = tt.cluster;

        if (tt.machineSets) {
          for (let i = 0; i < tt.machineSets.length ?? 0; i++) {
            const machineSet = tt.machineSets[i];

            if (state.value.machineSets.length <= i) {
              state.value.addMachineSet(LabelWorkerRole);
            }

            state.value.machineSets[i].machineAllocation = machineSet.machineAllocation;
            state.value.machineSets[i].machines = machineSet.machines ?? {};
            state.value.machineSets[i].patches = machineSet.patches ?? {};

            if (machineSet.bootstrapSpec) {
              state.value.machineSets[i].bootstrapSpec = machineSet.bootstrapSpec;
            }
          }
        }

        state.value.baseResources = tt.base;
        const resources = state.value.resources();

        expect(resources.length).toBeGreaterThan(0);

        const empty: Record<string, any> = {};

        for (const key in tt.expectedResources) {
          empty[key] = {};
        }

        for (const res of resources) {
          const expected = tt.expectedResources?.[res.metadata.type!]?.[res.metadata.id!];

          console.log(`should have resource ${res.metadata.type}/${res.metadata.id}`);

          expect(expected).toBeDefined();

          delete tt.expectedResources?.[res.metadata.type!]?.[res.metadata.id!];

          expect(res).toEqual({
            metadata: {
              id: res.metadata.id,
              namespace: DefaultNamespace,
              type: res.metadata.type,
              ...expected.metadata,
            },
            spec: expected.spec,
          });
        }

        expect(tt.expectedResources).toStrictEqual(empty);
      }

      if (tt.shouldFail) {
        expect(run).toThrowError();

        return;
      }

      run()
    });
  }
});
