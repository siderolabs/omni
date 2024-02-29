// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

export const parseLabels = (...labels: string[]): Record<string, string> => {
    const labelsMap: Record<string, string> = {};

    for (const label of labels) {
        const parts = label.split(":", 2);

        labelsMap[parts[0].trim()] = (parts[1] ?? "").trim();
    }

    return labelsMap;
}
