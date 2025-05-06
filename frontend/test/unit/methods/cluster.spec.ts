// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

import { describe, it, expect, mock, beforeEach } from "bun:test";
import { nextAvailableClusterName } from "../../../src/methods/cluster";
import { Resource } from "../../../src/api/v1alpha1/resource.pb";
import { ClusterSpec } from "../../../src/api/omni/specs/omni.pb";
import { Runtime } from "../../../src/api/common/omni.pb";
import { ResourceService } from "../../../src/api/grpc";
import { DefaultNamespace, ClusterType } from "../../../src/api/resources";
import { withRuntime } from "../../../src/api/options";

mock.module("../../../src/api/grpc", () => ({
  ResourceService: {
    List: mock(async () => []),
  },
}));

mock.module("../../../src/api/resources", () => ({
  DefaultNamespace: "default",
  ClusterType: "Cluster",
}));

mock.module("../../../src/api/options", () => ({
  withRuntime: mock((runtime: Runtime) => ({ runtime })),
}));

describe("nextAvailableClusterName", () => {
  const mockListClusters = (clusterIds: string[]) => {
    const mockResources: Resource<ClusterSpec>[] = clusterIds.map((id) => ({
      metadata: {
        id,
        type: ClusterType,
        namespace: DefaultNamespace,
        version: "1",
      },
      spec: {} as ClusterSpec,
    }));

    (ResourceService.List as any).mockImplementation(async () => mockResources);
  };

  beforeEach(() => {
    // Clear and reset mocks using type assertion
    (ResourceService.List as any).mockClear();
    (ResourceService.List as any).mockImplementation(async () => []);
    (withRuntime as any).mockClear();
  });

  it("should return the prefix if no clusters exist", async () => {
    mockListClusters([]);
    const name = await nextAvailableClusterName("test-cluster");

    expect(name).toBe("test-cluster");
    expect(ResourceService.List).toHaveBeenCalledWith(
      { namespace: DefaultNamespace, type: ClusterType },
      { runtime: Runtime.Omni }
    );
  });

  it("should return the prefix if it's not taken", async () => {
    mockListClusters(["another-cluster", "yet-another-cluster"]);
    const name = await nextAvailableClusterName("test-cluster");

    expect(name).toBe("test-cluster");
  });

  it("should return prefix-1 if prefix is taken and prefix-1 is available", async () => {
    mockListClusters(["test-cluster"]);
    const name = await nextAvailableClusterName("test-cluster");

    expect(name).toBe("test-cluster-1");
  });

  it("should return the next available number if prefix and some numbered versions are taken", async () => {
    mockListClusters(["test-cluster", "test-cluster-1", "test-cluster-2"]);
    const name = await nextAvailableClusterName("test-cluster");

    expect(name).toBe("test-cluster-3");
  });

  it("should handle a different prefix correctly", async () => {
    mockListClusters(["my-cluster", "my-cluster-1"]);
    const name = await nextAvailableClusterName("my-cluster");

    expect(name).toBe("my-cluster-2");
  });

  it("should return prefix-11 if prefix and prefix-1 through prefix-10 are taken", async () => {
    mockListClusters([
      "test-cluster",
      "test-cluster-1",
      "test-cluster-10",
      "test-cluster-2",
      "test-cluster-3",
      "test-cluster-4",
      "test-cluster-5",
      "test-cluster-6",
      "test-cluster-7",
      "test-cluster-8",
      "test-cluster-9",
    ]);
    const name = await nextAvailableClusterName("test-cluster");
    expect(name).toBe("test-cluster-11");
  });

  it("should call ResourceService.List with correct parameters and options", async () => {
    mockListClusters([]);
    await nextAvailableClusterName("test-prefix");

    expect(ResourceService.List).toHaveBeenCalledTimes(1);
    expect(ResourceService.List).toHaveBeenCalledWith(
      { namespace: DefaultNamespace, type: ClusterType },
      { runtime: Runtime.Omni }
    );
    expect(withRuntime).toHaveBeenCalledWith(Runtime.Omni);
  });

  it("should handle edge case when cluster names have numbers but in a different format", async () => {
    mockListClusters(["test-cluster", "test-cluster-abc"]);
    const name = await nextAvailableClusterName("test-cluster");

    expect(name).toBe("test-cluster-1");
  });
});
