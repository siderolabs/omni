// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { GrpcTunnelMode, type InstallationMediaConfigSpec } from '@/api/omni/specs/omni.pb'
import type { FormState } from '@/views/omni/InstallationMedia/InstallationMediaCreate.vue'

export function formStateToPreset(formState: FormState): InstallationMediaConfigSpec {
  return {
    architecture: formState.machineArch,
    bootloader: formState.bootloader,
    cloud:
      formState.hardwareType === 'cloud'
        ? {
            platform: formState.cloudPlatform,
          }
        : undefined,
    sbc:
      formState.hardwareType === 'sbc'
        ? {
            overlay: formState.sbcType,
            overlay_options: formState.overlayOptions,
          }
        : undefined,
    grpc_tunnel: formState.useGrpcTunnel ? GrpcTunnelMode.ENABLED : GrpcTunnelMode.DISABLED,
    talos_version: formState.talosVersion,
    install_extensions: formState.systemExtensions,
    join_token: formState.joinToken,
    kernel_args: formState.cmdline,
    machine_labels: formState.machineUserLabels
      ? Object.entries(formState.machineUserLabels).reduce<Record<string, string>>(
          (prev, [key, { value }]) => ({ ...prev, [key]: value }),
          {},
        )
      : undefined,
    secure_boot: formState.secureBoot,
  }
}
