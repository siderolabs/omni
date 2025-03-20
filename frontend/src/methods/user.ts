// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

import {
  DefaultNamespace,
  UserType,
  IdentityType,
  LabelIdentityUserID,
  RoleInfraProvider,
  ServiceAccountDomain,
  InfraProviderServiceAccountDomain,
} from "@/api/resources";

import {
  enums,
  generateKey,
} from 'openpgp/lightweight';

import { Resource, ResourceService } from "@/api/grpc";
import { UserSpec } from "@/api/omni/specs/auth.pb";
import { v4 as uuidv4 } from 'uuid';
import { IdentitySpec } from "@/api/omni/specs/auth.pb";
import { Runtime } from "@/api/common/omni.pb";
import { Code } from "@/api/google/rpc/code.pb";
import { withRuntime } from "@/api/options";
import { ManagementService } from "@/api/omni/management/management.pb";

export const createUser = async (email: string, role: string) => {
  const user: Resource<UserSpec> = {
    metadata: {
      id: uuidv4(),
      namespace: DefaultNamespace,
      type: UserType,
    },
    spec: {
      role: role
    }
  };

  const identity: Resource<IdentitySpec> = {
    metadata: {
      id: email.toLowerCase(),
      namespace: DefaultNamespace,
      type: IdentityType,
      labels: {
        [LabelIdentityUserID]: user.metadata.id as string,
      }
    },
    spec: {
      user_id: user.metadata.id as string,
    }
  }

  let identityExists = true;
  try {
    await ResourceService.Get({
      id: identity.metadata.id,
      namespace: DefaultNamespace,
      type: identity.metadata.type
    }, withRuntime(Runtime.Omni));
  } catch (e) {
    if (e.code !== Code.NOT_FOUND) {
      throw e;
    }

    identityExists = false;
  }

  if (identityExists) {
    throw new Error(`The email ${identity.metadata.id} is already in use`);
  }

  await ResourceService.Create(user, withRuntime(Runtime.Omni));

  await ResourceService.Create(identity, withRuntime(Runtime.Omni));
};

export const updateRole = async (userID: string, role: string) => {
  const user = await ResourceService.Get({
    id: userID,
    namespace: DefaultNamespace,
    type: UserType
  }, withRuntime(Runtime.Omni));

  user.spec.role = role;

  await ResourceService.Update(user, undefined, withRuntime(Runtime.Omni));
};

export const createServiceAccount = async (name: string, role: string, expirationDays: number = 365) => {
  const email = `${name}@${ role === RoleInfraProvider ? InfraProviderServiceAccountDomain : ServiceAccountDomain }`;

  const { privateKey, publicKey } = await generateKey({
    type: 'ecc',
    curve: 'ed25519Legacy',
    userIDs: [{ email: email }, ],
    keyExpirationTime: expirationDays * 24 * 60 * 60,
    config: {
      preferredCompressionAlgorithm: enums.compression.zlib,
      preferredSymmetricAlgorithm: enums.symmetric.aes256,
      preferredHashAlgorithm: enums.hash.sha256,
    }
  });

  await ManagementService.CreateServiceAccount({
    armored_pgp_public_key: publicKey,
    role,
    name: role === RoleInfraProvider ? `infra-provider:${ name }` : name,
  });

  const saKey = {
    name: name,
    pgp_key: privateKey.trim(),
  };

  const raw = JSON.stringify(saKey);

  return btoa(raw)
};

export const renewServiceAccount = async (id: string, expirationDays: number = 365) => {
  const { privateKey, publicKey } = await generateKey({
    type: 'ecc',
    curve: 'ed25519Legacy',
    userIDs: [{ email: id }, ],
    keyExpirationTime: expirationDays * 24 * 60 * 60,
    config: {
      preferredCompressionAlgorithm: enums.compression.zlib,
      preferredSymmetricAlgorithm: enums.symmetric.aes256,
      preferredHashAlgorithm: enums.hash.sha256,
    }
  });

  const parts = id.split("@");
  const name = parts[1] === InfraProviderServiceAccountDomain ? `infra-provider:${parts[0]}` : parts[0];

  await ManagementService.RenewServiceAccount({
    armored_pgp_public_key: publicKey,
    name: name
  });

  const saKey = {
    name: name,
    pgp_key: privateKey.trim(),
  };

  const raw = JSON.stringify(saKey);

  return btoa(raw)
};

