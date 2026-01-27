// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { GrpcTunnelMode, type InstallationMediaConfigSpec } from '@/api/omni/specs/omni.pb'
import type { LabelSelectItem } from '@/components/common/Labels/Labels.vue'
import type { FormState } from '@/views/omni/InstallationMedia/useFormState'

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

export function presetToFormState(preset: InstallationMediaConfigSpec): FormState {
  return {
    hardwareType: preset.cloud ? 'cloud' : preset.sbc ? 'sbc' : 'metal',
    machineArch: preset.architecture,
    bootloader: preset.bootloader,
    cloudPlatform: preset.cloud?.platform,
    sbcType: preset.sbc?.overlay,
    overlayOptions: preset.sbc?.overlay_options,
    useGrpcTunnel: preset.grpc_tunnel === GrpcTunnelMode.ENABLED,
    talosVersion: preset.talos_version,
    systemExtensions: preset.install_extensions,
    joinToken: preset.join_token,
    cmdline: preset.kernel_args,
    machineUserLabels: preset.machine_labels
      ? Object.entries(preset.machine_labels).reduce<Record<string, LabelSelectItem>>(
          (prev, [key, value]) => ({ ...prev, [key]: { value, canRemove: true } }),
          {},
        )
      : undefined,
    secureBoot: preset.secure_boot,
  }
}
