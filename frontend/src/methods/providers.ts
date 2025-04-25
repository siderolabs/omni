// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

import { Resource, ResourceService } from "@/api/grpc";
import { InfraProviderSpec } from "@/api/omni/specs/infra.pb";
import { InfraProviderNamespace, ProviderType, RoleInfraProvider } from "@/api/resources";
import { createServiceAccount } from "./user";
import { withRuntime } from "@/api/options";
import { Runtime } from "@/api/common/omni.pb";

export const setupInfraProvider = async (name: string) => {
  await ResourceService.Create<Resource<InfraProviderSpec>>({
    metadata: {
      id: name,
      namespace: InfraProviderNamespace,
      type: ProviderType,
    },
    spec: {}
  }, withRuntime(Runtime.Omni));

  return await createServiceAccount(name, RoleInfraProvider);
}
