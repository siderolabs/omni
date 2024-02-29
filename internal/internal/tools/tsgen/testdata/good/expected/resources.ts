// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// THIS FILE WAS AUTOMATICALLY GENERATED, PLEASE DO NOT EDIT.

export const Const2 = "const2";
export const ConstNotSolo = "constSolo";
export const ConstComputed = "constComputed";
export const NamespaceType = "Namespaces.meta.cosi.dev";
export const ResourceDefinitionType = "ResourceDefinitions.meta.cosi.dev";
export const MetaNamespace = "meta";

export const kubernetes = {
  service: "services.v1",
  pod: "pods.v1",
  node: "nodes.v1",
  cluster: `clusters`,
  machine: `machines`,
  sideroServers: "servers",
  crd: "customresourcedefinitions.v1.apiextensions.k8s.io",
};

export const talos = {
  // resources
  service: "Services.v1alpha1.talos.dev",
  cpu: "CPUStats.perf.talos.dev",
  mem: "MemoryStats.perf.talos.dev",
  nodename: "Nodenames.kubernetes.talos.dev",
  member: "Members.cluster.talos.dev",

  // well known resource IDs
  defaultNodeNameID: "nodename",

  // namespaces
  perfNamespace: "perf",
  clusterNamespace: "cluster",
  runtimeNamespace: "runtime",
  k8sNamespace: "k8s",
};
