// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

import { add } from 'date-fns'
import { enums, generateKey } from 'openpgp/lightweight'

import { ManagementService } from '@/api/omni/management/management.pb'
import {
  InfraProviderServiceAccountDomain,
  RoleInfraProvider,
  ServiceAccountDomain,
} from '@/api/resources'

export const createJoinToken = async (name: string, expirationDays?: number) => {
  let expirationTime: string | undefined

  if (expirationDays !== undefined) {
    expirationTime = add(new Date(), { days: expirationDays }).toISOString()
  }

  await ManagementService.CreateJoinToken({ expiration_time: expirationTime, name })
}

export const createServiceAccount = async (
  name: string,
  role: string,
  expirationDays: number = 365,
) => {
  const email = `${name}@${role === RoleInfraProvider ? InfraProviderServiceAccountDomain : ServiceAccountDomain}`

  const { privateKey, publicKey } = await generateKey({
    type: 'ecc',
    curve: 'ed25519Legacy',
    userIDs: [{ email: email }],
    keyExpirationTime: expirationDays * 24 * 60 * 60,
    config: {
      preferredCompressionAlgorithm: enums.compression.zlib,
      preferredSymmetricAlgorithm: enums.symmetric.aes256,
      preferredHashAlgorithm: enums.hash.sha256,
    },
  })

  await ManagementService.CreateServiceAccount({
    armored_pgp_public_key: publicKey,
    role,
    name: role === RoleInfraProvider ? `infra-provider:${name}` : name,
  })

  const saKey = {
    name: name,
    pgp_key: privateKey.trim(),
  }

  const raw = JSON.stringify(saKey)

  return btoa(raw)
}

export const renewServiceAccount = async (id: string, expirationDays: number = 365) => {
  const { privateKey, publicKey } = await generateKey({
    type: 'ecc',
    curve: 'ed25519Legacy',
    userIDs: [{ email: id }],
    keyExpirationTime: expirationDays * 24 * 60 * 60,
    config: {
      preferredCompressionAlgorithm: enums.compression.zlib,
      preferredSymmetricAlgorithm: enums.symmetric.aes256,
      preferredHashAlgorithm: enums.hash.sha256,
    },
  })

  const parts = id.split('@')
  const name =
    parts[1] === InfraProviderServiceAccountDomain ? `infra-provider:${parts[0]}` : parts[0]

  await ManagementService.RenewServiceAccount({
    armored_pgp_public_key: publicKey,
    name: name,
  })

  const saKey = {
    name: name,
    pgp_key: privateKey.trim(),
  }

  const raw = JSON.stringify(saKey)

  return btoa(raw)
}
