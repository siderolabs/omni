// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import Bowser from 'bowser'
import type { Node as V1Node } from 'kubernetes-types/core/v1'
import { coerce } from 'semver'
import type { Ref } from 'vue'
import { computed, ref } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import type { fetchOption } from '@/api/fetch.pb'
import { ManagementService } from '@/api/omni/management/management.pb'
import type { EtcdBackupOverallStatusSpec } from '@/api/omni/specs/omni.pb'
import { withContext } from '@/api/options'
import {
  DefaultTalosVersion,
  EtcdBackupOverallStatusID,
  EtcdBackupOverallStatusType,
  MetricsNamespace,
} from '@/api/resources'
import { NodesViewFilterOptions, TCommonStatuses } from '@/constants'
import { useResourceWatch } from '@/methods/useResourceWatch'
import { showError } from '@/notification'

export function getNonce() {
  return document.querySelector<HTMLMetaElement>("meta[name='csp-nonce']")?.content ?? ''
}

interface UADataValues {
  platform?: string
  architecture?: string
}

interface UADataNavigator extends Navigator {
  userAgentData?: {
    platform?: string
    getHighEntropyValues?: (hints: string[]) => Promise<UADataValues>
  }
}

export async function getPlatform() {
  try {
    const uaData = (navigator as UADataNavigator).userAgentData

    const { platform, architecture } = uaData?.getHighEntropyValues
      ? { platform: uaData.platform, ...(await uaData.getHighEntropyValues(['architecture'])) }
      : { platform: Bowser.parse(navigator.userAgent).os.name, architecture: undefined }

    const os = (() => {
      switch (platform?.toLowerCase()) {
        case 'macos':
          return 'darwin'
        case 'windows':
          return 'windows'
        default:
          return 'linux'
      }
    })()

    const arch = /arm/i.test(architecture ?? navigator.userAgent) ? 'arm64' : 'amd64'

    return [os, arch] as const
  } catch {
    return ['linux', 'amd64'] as const
  }
}

export const getStatus = (item: V1Node) => {
  const conditions = item?.status?.conditions
  if (!conditions) return TCommonStatuses.LOADING

  for (const c of conditions) {
    if (c.type === NodesViewFilterOptions.READY && c.status === 'True')
      return NodesViewFilterOptions.READY
  }

  return NodesViewFilterOptions.NOT_READY
}

export const downloadKubeconfig = async (cluster: string) => {
  try {
    const response = await ManagementService.Kubeconfig({}, withContext({ cluster }))

    downloadFile(
      `data:application/octet-stream;charset=utf-16le;base64,${response.kubeconfig}`,
      `${cluster}-kubeconfig.yaml`,
    )
  } catch (e) {
    showError('Failed to download Kubeconfig', e.message || e.toString())
  }
}

export const downloadTalosconfig = async (cluster?: string) => {
  const opts: fetchOption[] = []

  if (cluster) {
    opts.push(withContext({ cluster }))
  }

  try {
    const response = await ManagementService.Talosconfig({}, ...opts)

    downloadFile(
      `data:application/octet-stream;charset=utf-16le;base64,${response.talosconfig}`,
      cluster ? `${cluster}-talosconfig.yaml` : 'talosconfig.yaml',
    )
  } catch (e) {
    showError('Failed to download Talosconfig', e.message || e.toString())
  }
}

export const downloadOmniconfig = async () => {
  try {
    const response = await ManagementService.Omniconfig({})

    downloadFile(
      `data:application/octet-stream;charset=utf-16le;base64,${response.omniconfig}`,
      'omniconfig.yaml',
    )
  } catch (e) {
    showError('Failed to download omniconfig', e.message || e.toString())
  }
}

export const suspended = ref(false)
export const eulaAccepted = ref(false)

export enum AuthType {
  None = 0,
  Auth0 = 1,
  SAML = 2,
  OIDC = 3,
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

export const setupBackupStatus = () => {
  const { data: res } = useResourceWatch<EtcdBackupOverallStatusSpec>({
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
  }
}

export const isChrome = () => {
  return navigator.userAgent.toLowerCase().includes('chrome')
}

export const getKernelArgs = async (joinToken?: string, useGRPCTunnel: boolean = false) => {
  const response = await ManagementService.GetMachineJoinConfig({
    join_token: joinToken,
    use_grpc_tunnel: useGRPCTunnel,
  })

  return response.kernel_args?.join(' ') ?? ''
}

export const downloadMachineJoinConfig = async (
  joinToken?: string,
  useGRPCTunnel: boolean = false,
) => {
  const response = await ManagementService.GetMachineJoinConfig({
    join_token: joinToken,
    use_grpc_tunnel: useGRPCTunnel,
  })

  downloadFile(
    'data:text/plain;charset=utf-8,' + encodeURIComponent(response.config!),
    'machine-config.yaml',
  )
}

type DocsType = 'talos' | 'omni' | 'k8s'

export function getDocsLink(
  type: 'talos',
  path?: string,
  options?: { talosVersion?: string },
): string
export function getDocsLink(type: 'omni', path?: string): string
export function getDocsLink(type: 'k8s', path?: string): string
export function getDocsLink(type: DocsType, path?: string, options?: { talosVersion?: string }) {
  const parts = [getDocsBasePath(type)]

  if (type === 'talos') {
    parts.push(`v${majorMinorVersion(options?.talosVersion || DefaultTalosVersion)}`)
  }

  if (path) {
    parts.push(path.replace(/^\//, ''))
  }

  return parts.join('/')
}

function getDocsBasePath(type: DocsType) {
  const docsDomain = 'docs.siderolabs.com'

  switch (type) {
    case 'talos':
      return `https://${docsDomain}/talos`
    case 'omni':
      return `https://${docsDomain}/omni`
    case 'k8s':
      return `https://${docsDomain}/kubernetes-guides`
  }
}

export function majorMinorVersion(version: string) {
  const v = coerce(version)

  if (!v) {
    console.warn(`Invalid version "${version}" sent to majorMinVersion`)
    return '0.0'
  }

  return `${v.major}.${v.minor}`
}

export function downloadFile(url: string, filename?: string) {
  const a = document.createElement('a')
  a.style.display = 'none'

  a.href = url
  if (filename) a.download = filename

  document.body.appendChild(a)
  a.click()
  a.remove()
}
