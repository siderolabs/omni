// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { computedAsync } from '@vueuse/core'
import { dump } from 'js-yaml'
import { type MaybeRefOrGetter, ref, toValue } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import {
  type CreateSchematicRequestOverlay,
  CreateSchematicRequestSiderolinkGRPCTunnelMode,
  ManagementService,
} from '@/api/omni/management/management.pb'
import { GrpcTunnelMode, type InstallationMediaConfigSpec } from '@/api/omni/specs/omni.pb'
import { type SBCConfigSpec } from '@/api/omni/specs/virtual.pb'
import { LabelsMeta, SBCConfigType, VirtualNamespace } from '@/api/resources'
import { useResourceGet } from '@/methods/useResourceGet'

export function usePresetSchematic(presetRef: MaybeRefOrGetter<InstallationMediaConfigSpec>) {
  const { data: selectedSBC } = useResourceGet<SBCConfigSpec>(() => ({
    skip: !toValue(presetRef).sbc,
    runtime: Runtime.Omni,
    resource: {
      namespace: VirtualNamespace,
      type: SBCConfigType,
      id: toValue(presetRef).sbc?.overlay,
    },
  }))

  const schematicLoading = ref(false)
  const schematicError = ref<string>()

  const schematic = computedAsync(async () => {
    const preset = toValue(presetRef)

    let overlay: CreateSchematicRequestOverlay | undefined

    if (preset.sbc) {
      if (!selectedSBC.value) return

      overlay = {
        name: selectedSBC.value.spec.overlay_name,
        image: selectedSBC.value.spec.overlay_image,
        options: preset.sbc.overlay_options,
      }
    }

    try {
      schematicLoading.value = true

      const { schematic_id, schematic_yml } = await ManagementService.CreateSchematic({
        extensions: preset.install_extensions,
        extra_kernel_args: preset.kernel_args?.split(' ').filter((item) => item.trim()),
        meta_values: {
          [LabelsMeta]: dump({ machineLabels: preset.machine_labels ?? {} }),
        },
        overlay,
        talos_version: preset.talos_version,
        secure_boot: preset.secure_boot,
        siderolink_grpc_tunnel_mode:
          preset.grpc_tunnel === GrpcTunnelMode.ENABLED
            ? CreateSchematicRequestSiderolinkGRPCTunnelMode.ENABLED
            : CreateSchematicRequestSiderolinkGRPCTunnelMode.DISABLED,
        join_token: preset.join_token,
        bootloader: preset.bootloader,
      })

      return {
        id: schematic_id,
        yml: schematic_yml,
      }
    } catch (error) {
      schematicError.value = error?.message || String(error)
    } finally {
      schematicLoading.value = false
    }
  })

  return { schematic, schematicError, schematicLoading }
}
