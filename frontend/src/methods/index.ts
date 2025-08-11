// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

import type { V1Node } from '@kubernetes/client-node'
import type { ComputedRef, Ref } from 'vue'
import { computed, ref } from 'vue'
import { copyText } from 'vue3-clipboard'

import { Runtime } from '@/api/common/omni.pb'
import type { fetchOption } from '@/api/fetch.pb'
import { b64Decode } from '@/api/fetch.pb'
import type { Resource } from '@/api/grpc'
import { ManagementService } from '@/api/omni/management/management.pb'
import type { EtcdBackupOverallStatusSpec } from '@/api/omni/specs/omni.pb'
import { withContext } from '@/api/options'
import {
  EtcdBackupOverallStatusID,
  EtcdBackupOverallStatusType,
  MetricsNamespace,
} from '@/api/resources'
import Watch from '@/api/watch'
import { NodesViewFilterOptions, TCommonStatuses } from '@/constants'
import { showError } from '@/notification'

export const getStatus = (item: V1Node) => {
  const conditions = item?.status?.conditions
  if (!conditions) return TCommonStatuses.LOADING

  for (const c of conditions) {
    if (c.type === NodesViewFilterOptions.READY && c.status === 'True')
      return NodesViewFilterOptions.READY
  }

  return NodesViewFilterOptions.NOT_READY
}

export const getServiceHealthStatus = (service) => {
  return service?.spec?.unknown
    ? TCommonStatuses.HEALTH_UNKNOWN
    : service?.spec?.healthy
      ? TCommonStatuses.READY
      : TCommonStatuses.UNHEALTHY
}

export const cpuParser = (input) => {
  const milliMatch = input.match(/^([0-9]+)m$/)
  if (milliMatch) {
    return milliMatch[1] / 1000
  }

  return parseFloat(input)
}

const memoryMultipliers = {
  k: 1000,
  M: 1000 ** 2,
  G: 1000 ** 3,
  T: 1000 ** 4,
  P: 1000 ** 5,
  E: 1000 ** 6,
  Ki: 1024,
  Mi: 1024 ** 2,
  Gi: 1024 ** 3,
  Ti: 1024 ** 4,
  Pi: 1024 ** 5,
  Ei: 1024 ** 6,
}

export const memoryParser = (input) => {
  const unitMatch = input.match(/^([0-9]+)([A-Za-z]{1,2})$/)
  if (unitMatch) {
    return parseInt(unitMatch[1], 10) * memoryMultipliers[unitMatch[2]]
  }

  return parseInt(input, 10)
}

export const formatBytes = (bytes, decimals = 2) => {
  if (!bytes) return '0 Bytes'

  const k = 1024
  const dm = decimals < 0 ? 0 : decimals
  const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB', 'PB', 'EB', 'ZB', 'YB']

  const i = Math.floor(Math.log(bytes) / Math.log(k))

  return (bytes / Math.pow(k, i)).toFixed(dm) + ' ' + sizes[i]
}

export const downloadKubeconfig = async (cluster: string) => {
  const link = document.createElement('a')
  try {
    const response = await ManagementService.Kubeconfig({}, withContext({ cluster }))

    link.href = `data:application/octet-stream;charset=utf-16le;base64,${response.kubeconfig}`
    link.download = `${cluster}-kubeconfig.yaml`
    link.click()
  } catch (e) {
    showError('Failed to download Kubeconfig', e.message || e.toString())
  }
}

export const downloadTalosconfig = async (cluster?: string) => {
  const link = document.createElement('a')
  const opts: fetchOption[] = []

  if (cluster) {
    opts.push(withContext({ cluster }))
  }

  try {
    const response = await ManagementService.Talosconfig({}, ...opts)

    link.href = `data:application/octet-stream;charset=utf-16le;base64,${response.talosconfig}`
    link.download = cluster ? `${cluster}-talosconfig.yaml` : 'talosconfig.yaml'
    link.click()
  } catch (e) {
    showError('Failed to download Talosconfig', e.message || e.toString())
  }
}

export const downloadOmniconfig = async () => {
  const link = document.createElement('a')
  try {
    const response = await ManagementService.Omniconfig({})

    link.href = `data:application/octet-stream;charset=utf-16le;base64,${response.omniconfig}`
    link.download = 'omniconfig.yaml'
    link.click()
  } catch (e) {
    showError('Failed to download omniconfig', e.message || e.toString())
  }
}

export const downloadAuditLog = async () => {
  try {
    const result: Uint8Array[] = []

    await ManagementService.ReadAuditLog({}, (resp) => {
      const data = resp.audit_log as unknown as string // audit_log is actually not a Uint8Array, but a base64 string

      result.push(b64Decode(data))
    })

    const link = document.createElement('a')
    link.href = window.URL.createObjectURL(new Blob(result, { type: 'application/json' }))
    link.download = 'auditlog.jsonlog'
    link.click()
  } catch (e) {
    showError('Failed to download audit log', e.error?.message ?? e.message ?? e.toString())
  }
}

export const suspended = ref(false)

export enum AuthType {
  None = 0,
  Auth0 = 1,
  SAML = 2,
}

export const authType: Ref<AuthType> = ref(AuthType.None)

export type BackupsStatus = {
  enabled: boolean
  error?: string
  configurable?: boolean
  store?: string
}

const capitalize = (w: string) => {
  return `${w.charAt(0).toUpperCase()}${w.slice(1)}`
}

export const setupBackupStatus = (): {
  status: ComputedRef<BackupsStatus>
  watch: Watch<Resource<EtcdBackupOverallStatusSpec>>
} => {
  const res = ref<Resource<EtcdBackupOverallStatusSpec>>()

  const watch = new Watch<Resource<EtcdBackupOverallStatusSpec>>(res)

  watch.setup({
    resource: {
      id: EtcdBackupOverallStatusID,
      namespace: MetricsNamespace,
      type: EtcdBackupOverallStatusType,
    },
    runtime: Runtime.Omni,
  })
  return {
    status: computed(() => {
      const configurable = res.value?.spec.configuration_name === 's3'

      if (res.value?.spec.configuration_error) {
        return {
          error: `${capitalize(res.value.spec.configuration_name!)} ${res.value.spec.configuration_error}`,
          enabled: false,
          configurable,
          store: res.value.spec.configuration_name,
        }
      }

      return {
        enabled: true,
        configurable,
        store: res.value?.spec.configuration_name,
      }
    }),
    watch,
  }
}

export const isChrome = () => {
  return navigator.userAgent.toLowerCase().includes('chrome')
}

export const copyKernelArgs = async (joinToken?: string, useGRPCTunnel: boolean = false) => {
  const response = await ManagementService.GetMachineJoinConfig({
    join_token: joinToken,
    use_grpc_tunnel: useGRPCTunnel,
  })

  copyText(response.kernel_args!.join(' '), undefined, () => {})
}

export const downloadMachineJoinConfig = async (
  joinToken?: string,
  useGRPCTunnel: boolean = false,
) => {
  const response = await ManagementService.GetMachineJoinConfig({
    join_token: joinToken,
    use_grpc_tunnel: useGRPCTunnel,
  })

  const element = document.createElement('a')
  element.setAttribute(
    'href',
    'data:text/plain;charset=utf-8,' + encodeURIComponent(response.config!),
  )
  element.setAttribute('download', 'machine-config.yaml')

  element.style.display = 'none'
  document.body.appendChild(element)

  element.click()

  document.body.removeChild(element)
}
