// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

import { add, type Duration, milliseconds, millisecondsToSeconds } from 'date-fns'
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
  expiration: Duration = { days: 365 },
) => {
  const email = `${name}@${role === RoleInfraProvider ? InfraProviderServiceAccountDomain : ServiceAccountDomain}`

  const { privateKey, publicKey } = await generateKey({
    type: 'ecc',
    curve: 'ed25519Legacy',
    userIDs: [{ email: email }],
    keyExpirationTime: millisecondsToSeconds(milliseconds(expiration)),
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

// Converts a service account identity into the name the management API expects.
const getServiceAccountName = (id: string) => {
  const [name, domain] = id.split('@')

  return domain === InfraProviderServiceAccountDomain ? `infra-provider:${name}` : name
}

export const renewServiceAccount = async (id: string, duration: Duration = { days: 365 }) => {
  const { privateKey, publicKey } = await generateKey({
    type: 'ecc',
    curve: 'ed25519Legacy',
    userIDs: [{ email: id }],
    keyExpirationTime: millisecondsToSeconds(milliseconds(duration)),
    config: {
      preferredCompressionAlgorithm: enums.compression.zlib,
      preferredSymmetricAlgorithm: enums.symmetric.aes256,
      preferredHashAlgorithm: enums.hash.sha256,
    },
  })

  const name = getServiceAccountName(id)

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

export const revokeServiceAccountKey = async (id: string, publicKeyId: string) => {
  await ManagementService.RevokeServiceAccountKey({
    name: getServiceAccountName(id),
    public_key_id: publicKeyId,
  })
}
