// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { describe, expect, it } from 'vitest'

import { SchematicBootloader } from '@/api/omni/management/management.pb'
import { GrpcTunnelMode, type InstallationMediaConfigSpec } from '@/api/omni/specs/omni.pb'
import { PlatformConfigSpecArch } from '@/api/omni/specs/virtual.pb'
import type { FormState } from '@/views/omni/InstallationMedia/useFormState'

import { formStateToPreset, presetToFormState } from './formStateToPreset'

describe('form<->preset conversion', () => {
  it('should convert correctly for minimum arguments', () => {
    expect(
      formStateToPreset({
        hardwareType: 'metal',
        talosVersion: 'v1.12.0',
        machineArch: PlatformConfigSpecArch.AMD64,
      }),
    ).toEqual({
      talos_version: 'v1.12.0',
      architecture: PlatformConfigSpecArch.AMD64,
      grpc_tunnel: GrpcTunnelMode.DISABLED,
    })

    expect(
      presetToFormState({
        talos_version: 'v1.12.0',
        architecture: PlatformConfigSpecArch.AMD64,
      }),
    ).toEqual({
      hardwareType: 'metal',
      talosVersion: 'v1.12.0',
      machineArch: PlatformConfigSpecArch.AMD64,
      useGrpcTunnel: false,
    })
  })

  it('should convert correctly for cloud', () => {
    const formState: FormState = {
      hardwareType: 'cloud',
      machineArch: PlatformConfigSpecArch.AMD64,
      bootloader: SchematicBootloader.BOOT_AUTO,
      cloudPlatform: 'aws',
      useGrpcTunnel: true,
      talosVersion: 'v1.7.0',
      systemExtensions: ['siderolabs/tailscale'],
      joinToken: 'test-token',
      cmdline: 'net.ifnames=0',
      machineUserLabels: {
        'user/label': { value: 'test', canRemove: true },
      },
      secureBoot: true,
    }

    const preset: InstallationMediaConfigSpec = {
      architecture: PlatformConfigSpecArch.AMD64,
      bootloader: SchematicBootloader.BOOT_AUTO,
      cloud: {
        platform: 'aws',
      },
      grpc_tunnel: GrpcTunnelMode.ENABLED,
      talos_version: 'v1.7.0',
      install_extensions: ['siderolabs/tailscale'],
      join_token: 'test-token',
      kernel_args: 'net.ifnames=0',
      machine_labels: {
        'user/label': 'test',
      },
      secure_boot: true,
    }

    expect(formStateToPreset(formState)).toEqual(preset)
    expect(presetToFormState(preset)).toEqual(formState)
  })

  it('should convert correctly for sbc', () => {
    const formState: FormState = {
      hardwareType: 'sbc',
      machineArch: PlatformConfigSpecArch.ARM64,
      bootloader: SchematicBootloader.BOOT_SD,
      sbcType: 'rpi_4',
      overlayOptions: 'console=ttyAMA0,115200',
      useGrpcTunnel: false,
      talosVersion: 'v1.7.0',
      systemExtensions: ['siderolabs/tailscale'],
      joinToken: 'test-token',
      cmdline: 'net.ifnames=0',
      machineUserLabels: {
        'user/label': { value: 'test', canRemove: true },
      },
      secureBoot: false,
    }

    const preset: InstallationMediaConfigSpec = {
      architecture: PlatformConfigSpecArch.ARM64,
      bootloader: SchematicBootloader.BOOT_SD,
      sbc: {
        overlay: 'rpi_4',
        overlay_options: 'console=ttyAMA0,115200',
      },
      grpc_tunnel: GrpcTunnelMode.DISABLED,
      talos_version: 'v1.7.0',
      install_extensions: ['siderolabs/tailscale'],
      join_token: 'test-token',
      kernel_args: 'net.ifnames=0',
      machine_labels: {
        'user/label': 'test',
      },
      secure_boot: false,
    }

    expect(formStateToPreset(formState)).toEqual(preset)
    expect(presetToFormState(preset)).toEqual(formState)
  })

  it('should convert correctly for metal', () => {
    const formState: FormState = {
      hardwareType: 'metal',
      machineArch: PlatformConfigSpecArch.AMD64,
      bootloader: SchematicBootloader.BOOT_AUTO,
      useGrpcTunnel: true,
      talosVersion: 'v1.7.0',
      systemExtensions: ['siderolabs/tailscale'],
      joinToken: 'test-token',
      cmdline: 'net.ifnames=0',
      machineUserLabels: {
        'user/label': { value: 'test', canRemove: true },
      },
      secureBoot: false,
    }

    const preset: InstallationMediaConfigSpec = {
      architecture: PlatformConfigSpecArch.AMD64,
      bootloader: SchematicBootloader.BOOT_AUTO,
      grpc_tunnel: GrpcTunnelMode.ENABLED,
      talos_version: 'v1.7.0',
      install_extensions: ['siderolabs/tailscale'],
      join_token: 'test-token',
      kernel_args: 'net.ifnames=0',
      machine_labels: {
        'user/label': 'test',
      },
      secure_boot: false,
    }

    expect(formStateToPreset(formState)).toEqual(preset)
    expect(presetToFormState(preset)).toEqual(formState)
  })
})
