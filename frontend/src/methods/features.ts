// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

import { computed, Ref, ref } from "vue";
import { Resource } from "@/api/grpc";
import { FeaturesConfigSpec } from "@/api/omni/specs/omni.pb";
import Watch from "@/api/watch";
import { Runtime } from "@/api/common/omni.pb";
import { FeaturesConfigType, DefaultNamespace, FeaturesConfigID } from "@/api/resources";

export const setupWorkloadProxyingEnabledFeatureWatch = (): Ref<boolean> => {
  const featuresConfig: Ref<Resource<FeaturesConfigSpec> | undefined> = ref();
  const featuresConfigWatch = new Watch(featuresConfig);
  featuresConfigWatch.setup({
    resource: {
      type: FeaturesConfigType,
      namespace: DefaultNamespace,
      id: FeaturesConfigID,
    },
    runtime: Runtime.Omni,
  });

  return computed(() => {
    return featuresConfig?.value?.spec?.enable_workload_proxying ?? false;
  });
}
