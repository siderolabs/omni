// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

import { Runtime } from "@/api/common/omni.pb";
import { Code } from "@/api/google/rpc/code.pb";

import { Resource, ResourceService } from "@/api/grpc";
import { InfraMachineConfigSpec, InfraMachineConfigSpecAcceptanceStatus, MachineLabelsSpec } from "@/api/omni/specs/omni.pb";
import { withContext, withRuntime } from "@/api/options";
import { DefaultNamespace, InfraMachineConfigType, MachineLabelsType, MachineLocked, MachineSetNodeType, MachineStatusType, SiderolinkResourceType, SystemLabelPrefix } from "@/api/resources";
import { MachineService } from "@/api/talos/machine/machine.pb";
import { destroyResources, getMachineConfigPatchesToDelete } from "@/methods/cluster";
import { parseLabels } from "@/methods/labels";
import { getImageFactoryBaseURL } from "@/methods/features";

export const addMachineLabels = async (machineID: string, ...labels: string[]) => {
  let resource: Resource = {
    metadata: {
      type: MachineLabelsType,
      namespace: DefaultNamespace,
      id: machineID
    },
    spec: {},
  };

  let exists = true;
  try {
    resource = await ResourceService.Get(resource.metadata,
      withRuntime(Runtime.Omni),
    );
  } catch (e) {
    if (e.code !== Code.NOT_FOUND) {
      throw e;
    }

    exists = false;
  }

  resource.metadata.labels = {
    ...resource.metadata.labels,
    ...parseLabels(...labels)
  };

  if (exists) {
    await ResourceService.Update(resource, resource.metadata.version, withRuntime(
      Runtime.Omni,
    ));
  } else {
    const machine = await ResourceService.Get({
      type: MachineStatusType,
      namespace: DefaultNamespace,
      id: machineID,
    }, withRuntime(Runtime.Omni))

    copyUserLabels(machine, resource);

    await ResourceService.Create(resource, withRuntime(Runtime.Omni));
  }
};

export const removeMachineLabels = async (machineID: string, ...keys: string[]) => {
  let resource: Resource<MachineLabelsSpec>;
  const metadata = {
    id: machineID,
    type: MachineLabelsType,
    namespace: DefaultNamespace,
  };

  try {
    resource = await ResourceService.Get(metadata, withRuntime(Runtime.Omni));
  } catch (e) {
    if (e.code !== Code.NOT_FOUND) {
      throw e;
    }

    resource = {
      metadata,
      spec: {},
    }

    const machineStatus = await ResourceService.Get({...metadata, type: MachineStatusType}, withRuntime(Runtime.Omni));
    copyUserLabels(machineStatus, resource);

    await ResourceService.Create(resource, withRuntime(Runtime.Omni));
  }

  if (!resource.metadata.labels) {
    return;
  }

  for (const key of keys) {
    delete (resource.metadata.labels[key]);
  }

  if (Object.keys(resource.metadata.labels).length === 0) {
    await ResourceService.Delete({
      id: resource.metadata.id,
      type: resource.metadata.type,
      namespace: resource.metadata.namespace,
    }, withRuntime(Runtime.Omni));
  } else {
    await ResourceService.Update(resource, undefined, withRuntime(Runtime.Omni));
  }
}

export const removeMachine = async (id: string) => {
  await ResourceService.Teardown({
    namespace: DefaultNamespace,
    type: SiderolinkResourceType,
    id: id,
  }, withRuntime(Runtime.Omni));

  const patches = await getMachineConfigPatchesToDelete(id);
  await destroyResources(patches);
}

export const updateMachineLock = async (id: string, locked: boolean) => {
  const machine = await ResourceService.Get({
    namespace: DefaultNamespace,
    type: MachineSetNodeType,
    id: id,
  }, withRuntime(Runtime.Omni));

  if (!machine.metadata.annotations)
    machine.metadata.annotations = {}

  if (locked) {
    machine.metadata.annotations[MachineLocked] = "";
  } else {
    delete machine.metadata.annotations[MachineLocked];
  }

  await ResourceService.Update(machine, undefined, withRuntime(Runtime.Omni));
}

const copyUserLabels = (src: Resource, dst: Resource) => {
  if (src.metadata.labels) {
    for (const key in src.metadata.labels) {
      if (key.indexOf(SystemLabelPrefix) === 0) {
        continue;
      }

      if (!dst.metadata.labels) {
        dst.metadata.labels = {};
      }

      dst.metadata.labels[key] = src.metadata.labels[key];
    }
  }
}

export const updateTalosMaintenance = async (machine: string, talosVersion: string, schematic?: string) => {
  const imageFactoryBaseURL = await getImageFactoryBaseURL();

  const host = new URL(imageFactoryBaseURL).host;

  const image = schematic ?
    `${host}/installer/${schematic}:v${talosVersion}` :
    `ghcr.io/siderolabs/installer:v${talosVersion}`;

  await MachineService.Upgrade({image}, withRuntime(Runtime.Talos), withContext({
    nodes: [machine]
  }));
}

export const rejectMachine = async (machine: string) => {
  await updateInfraMachineConfig(machine, (r: Resource<InfraMachineConfigSpec>) => {
    r.spec.acceptance_status = InfraMachineConfigSpecAcceptanceStatus.REJECTED;
  });
};

export const acceptMachine = async (machine: string) => {
  await updateInfraMachineConfig(machine, (r: Resource<InfraMachineConfigSpec>) => {
    r.spec.acceptance_status = InfraMachineConfigSpecAcceptanceStatus.ACCEPTED;
  });
};

export const updateInfraMachineConfig = async (machine: string, modify: (r: Resource<InfraMachineConfigSpec>) => void) => {
  const metadata = {
    id: machine,
    namespace: DefaultNamespace,
    type: InfraMachineConfigType,
  };

  try {
    const resource: Resource<InfraMachineConfigSpec> = await ResourceService.Get(metadata, withRuntime(Runtime.Omni));

    modify(resource);

    ResourceService.Update(resource, resource.metadata.version, withRuntime(Runtime.Omni))
  } catch (e) {
    if (e.code === Code.NOT_FOUND) {
      const resource: Resource<InfraMachineConfigSpec> = {
        metadata,
        spec: {}
      };

      modify(resource);

      await ResourceService.Create<Resource<InfraMachineConfigSpec>>(resource, withRuntime(Runtime.Omni));
    }
  }
}

export enum MachineFilterOption {
  Manual = "manual",
  Unaccepted = "unaccepted",
  Provisioned = "provisioned",
  PXE = "pxe",
};
