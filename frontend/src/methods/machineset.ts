// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

import { Runtime } from "@/api/common/omni.pb";
import { ResourceService } from "@/api/grpc";
import { MachineSetSpecMachineClassAllocationType } from "@/api/omni/specs/omni.pb";
import { withRuntime } from "@/api/options";
import { ControlPlanesIDSuffix, DefaultNamespace, DefaultWorkersIDSuffix, MachineSetType } from "@/api/resources";

export const controlPlaneTitle = "Control Planes";
export const workersTitlePrefix = "Workers: ";
export const defaultWorkersTitle = `${workersTitlePrefix}default`;

export const controlPlaneMachineSetId = (clusterId: string) => `${clusterId}-${ControlPlanesIDSuffix}`;

export const defaultWorkersMachineSetId = (clusterId: string) => `${clusterId}-${DefaultWorkersIDSuffix}`;

// sortMachineSetIds sorts the given machine set ids in the following order:
// 1. Control Plane
// 2. Default Workers - machine set which follows the form of "<cluster id>-workers"
// 3-n. Other Workers - machine sets which follow the form of "<cluster id>-<name>", sorted alphabetically by id.
export const sortMachineSetIds = (clusterId: string | undefined, ids: string[]) : string[] => {
  if (!clusterId) {
    return ids;
  }

  const idsCopy = ids.concat();
  return idsCopy.sort((a, b) => {
    const nameA = machineSetName(clusterId, a);
    const nameB = machineSetName(clusterId, b);

    if (nameA == ControlPlanesIDSuffix) {
      return -1;
    }

    if (nameB == ControlPlanesIDSuffix) {
      return 1;
    }

    if (nameA == DefaultWorkersIDSuffix) {
      return -1;
    }

    if (nameB == DefaultWorkersIDSuffix) {
      return 1;
    }

    return nameA.localeCompare(nameB);
  });
}

// machineSetTitle strips the cluster id prefix from the machine set id and returns a human-readable
// representation of the machine set id, such as "Control Plane", "Workers", "Workers: additional", etc.
export const machineSetTitle = (clusterId?: string, id?: string) => {
  const name = machineSetName(clusterId, id);
  if (!name) {
    return "";
  }

  if (name == ControlPlanesIDSuffix) {
    return controlPlaneTitle;
  }

  if (name == DefaultWorkersIDSuffix) {
    return defaultWorkersTitle;
  }

  return `${workersTitlePrefix}${name}`;
}

const machineSetName = (clusterId?: string, id?: string) => {
  if (!clusterId || !id) {
    return "";
  }

  if (!id.startsWith(clusterId + "-")) {
    return id;
  }

  return id.substring(clusterId.length + 1);
}

export const scaleMachineSet = async (id: string, machineCount: number, allocationType: MachineSetSpecMachineClassAllocationType) => {
  if (machineCount < 0) {
    throw new Error("machine set count can not be negative");
  }

  const ms = await ResourceService.Get({
    id: id,
    type: MachineSetType,
    namespace: DefaultNamespace,
  }, withRuntime(Runtime.Omni));

  if (!ms.spec.machine_class.name) {
    throw new Error("machine set does not use machine classes");
  }

  if (allocationType !== MachineSetSpecMachineClassAllocationType.Static) {
    machineCount = 0;
  }

  ms.spec.machine_class.machine_count = machineCount;
  ms.spec.machine_class.allocation_type = allocationType;

  await ResourceService.Update(ms, ms.metadata.version, withRuntime(Runtime.Omni));
}
