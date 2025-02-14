// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

import { InfraProviderLabelPrefix, LabelInfraProviderID, SystemLabelPrefix } from "@/api/resources";

export const parseLabels = (...labels: string[]): Record<string, string> => {
    const labelsMap: Record<string, string> = {};

    for (const label of labels) {
        const parts = label.split(":", 2);

        labelsMap[parts[0].trim()] = (parts[1] ?? "").trim();
    }

    return labelsMap;
}

export type Label = {
  key: string
  id: string
  value: string
  color: string
  removable?: boolean
  description?: string
  icon?: string
}

const labelColors = {
  "cluster": "light1",
  "available": "yellow",
  "invalid-state": "red",
  "connected": "green",
  "disconnected": "red",
  "platform": "blue1",
  "cores": "cyan",
  "mem": "blue2",
  "storage": "violet",
  "net": "blue3",
  "cpu": "orange",
  "arch": "light2",
  "region": "light3",
  "zone": "light4",
  "instance": "light5",
  "talos-version": "light7",
  "role-controlplane": "yellow",
  "role-worker": "yellow",
}

export const getLabelColor = (labelKey: string) => {
  return labelColors[labelKey] ?? "light6";
}

export const addLabel = (dest: Label[], label: Label) => {
  if (dest.find(l => l.value === label.value && l.key === label.key)) {
    return;
  }

  dest.push(
    {
      ...label,
      id: !label.value ? `has label: ${label.id}` : label.id,
    },
  );
}

export const selectors = (labels: Label[]) => {
  if (labels.length === 0) {
    return;
  }

  return labels.map((label: Label) => {
    if (label.value === "") {
      return label.key;
    }

    return `${label.key}=${label.value}`;
  });
}

const labelDescriptions = {
  "invalid-state": "The machine is expected to be unallocated, but still has the configuration of a cluster.\nIt might be required to wipe the machine bypassing Omni."
};

export const getLabelFromID = (key: string, value: string): Label => {
  const isUser = key.indexOf(SystemLabelPrefix) !== 0;

  let strippedKey = key.replace(new RegExp(`^${SystemLabelPrefix}`), "");
  let icon: string | undefined;
  let color = getLabelColor(strippedKey);
  let description = labelDescriptions[strippedKey];

  if (key.indexOf(InfraProviderLabelPrefix) === 0 && key !== LabelInfraProviderID) {
    const parts = strippedKey.split("/");
    strippedKey = parts[parts.length - 1];

    icon = "server-network";
    color = "green";

    description = `Defined by the infra provider "${parts[1]}""`
  }

  return {
    key: key,
    id: strippedKey,
    value: value,
    color: color,
    removable: isUser,
    description: description,
    icon: icon,
  };
}
