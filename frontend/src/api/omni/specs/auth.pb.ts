/* eslint-disable */
// @ts-nocheck
/*
* This file is a generated Typescript file for GRPC Gateway, DO NOT MODIFY
*/

import * as GoogleProtobufTimestamp from "../../google/protobuf/timestamp.pb"

export enum PublicKeySpecType {
  UNKNOWN = 0,
  PGP = 1,
  PLAIN = 2,
}

export type AuthConfigSpecAuth0 = {
  enabled?: boolean
  domain?: string
  client_id?: string
  useFormData?: boolean
}

export type AuthConfigSpecOIDC = {
  enabled?: boolean
  provider_url?: string
  client_id?: string
  client_secret?: string
  scopes?: string[]
}

export type AuthConfigSpecWebauthn = {
  enabled?: boolean
  required?: boolean
}

export type AuthConfigSpecSAML = {
  enabled?: boolean
  url?: string
  metadata?: string
  label_rules?: {[key: string]: string}
  name_id_format?: string
  attribute_rules?: {[key: string]: string}
}

export type AuthConfigSpec = {
  auth0?: AuthConfigSpecAuth0
  webauthn?: AuthConfigSpecWebauthn
  suspended?: boolean
  saml?: AuthConfigSpecSAML
  oidc?: AuthConfigSpecOIDC
}

export type SAMLAssertionSpec = {
  data?: Uint8Array
  email?: string
  used?: boolean
}

export type UserSpec = {
  scopes?: string[]
  role?: string
}

export type IdentitySpec = {
  user_id?: string
}

export type Identity = {
  email?: string
}

export type PublicKeySpec = {
  public_key?: Uint8Array
  scopes?: string[]
  expiration?: GoogleProtobufTimestamp.Timestamp
  confirmed?: boolean
  identity?: Identity
  role?: string
  type?: PublicKeySpecType
}

export type AccessPolicyUserGroupUser = {
  name?: string
  match?: string
  label_selectors?: string[]
}

export type AccessPolicyUserGroup = {
  users?: AccessPolicyUserGroupUser[]
}

export type AccessPolicyClusterGroupCluster = {
  name?: string
  match?: string
}

export type AccessPolicyClusterGroup = {
  clusters?: AccessPolicyClusterGroupCluster[]
}

export type AccessPolicyRuleKubernetesImpersonate = {
  groups?: string[]
}

export type AccessPolicyRuleKubernetes = {
  impersonate?: AccessPolicyRuleKubernetesImpersonate
}

export type AccessPolicyRule = {
  users?: string[]
  clusters?: string[]
  kubernetes?: AccessPolicyRuleKubernetes
  role?: string
}

export type AccessPolicyTestExpectedKubernetesImpersonate = {
  groups?: string[]
}

export type AccessPolicyTestExpectedKubernetes = {
  impersonate?: AccessPolicyTestExpectedKubernetesImpersonate
}

export type AccessPolicyTestExpected = {
  kubernetes?: AccessPolicyTestExpectedKubernetes
  role?: string
}

export type AccessPolicyTestUser = {
  name?: string
  labels?: {[key: string]: string}
}

export type AccessPolicyTestCluster = {
  name?: string
}

export type AccessPolicyTest = {
  name?: string
  user?: AccessPolicyTestUser
  cluster?: AccessPolicyTestCluster
  expected?: AccessPolicyTestExpected
}

export type AccessPolicySpec = {
  user_groups?: {[key: string]: AccessPolicyUserGroup}
  cluster_groups?: {[key: string]: AccessPolicyClusterGroup}
  rules?: AccessPolicyRule[]
  tests?: AccessPolicyTest[]
}

export type SAMLLabelRuleSpec = {
  match_labels?: string[]
  assign_role_on_registration?: string
  assign_role?: string
  update_on_each_login?: boolean
}

export type ServiceAccountStatusSpecPgpPublicKey = {
  id?: string
  armored?: string
  expiration?: GoogleProtobufTimestamp.Timestamp
}

export type ServiceAccountStatusSpec = {
  role?: string
  public_keys?: ServiceAccountStatusSpecPgpPublicKey[]
}