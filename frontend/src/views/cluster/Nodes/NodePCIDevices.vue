<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script lang="ts">
export interface TalosPCIDeviceSpec {
  class?: string
  subclass?: string
  vendor?: string
  product?: string
  class_id?: string
  subclass_id?: string
  vendor_id?: string
  product_id?: string
  driver?: string
}
</script>
<script setup lang="ts">
import { Runtime } from '@/api/common/omni.pb'
import { TalosHardwareNamespace, TalosPCIDeviceType } from '@/api/resources'
import { itemID } from '@/api/watch'
import TSpinner from '@/components/common/Spinner/TSpinner.vue'
import TableCell from '@/components/common/Table/TableCell.vue'
import TableRoot from '@/components/common/Table/TableRoot.vue'
import TableRow from '@/components/common/Table/TableRow.vue'
import TAlert from '@/components/TAlert.vue'
import { getContext } from '@/context'
import { useResourceWatch } from '@/methods/useResourceWatch'

const {
  data: devices,
  loading,
  err,
} = useResourceWatch<TalosPCIDeviceSpec>({
  resource: {
    namespace: TalosHardwareNamespace,
    type: TalosPCIDeviceType,
  },
  runtime: Runtime.Talos,
  context: getContext(),
})
</script>

<template>
  <div class="py-4">
    <TSpinner v-if="loading" class="size-4" />
    <TAlert v-else-if="err" type="error" title="Error">{{ err }}</TAlert>
    <TAlert v-else-if="!devices.length" type="info" title="No Records">
      No entries of the requested resource type are found on the server.
    </TAlert>

    <TableRoot v-else class="w-full min-w-max overflow-x-auto">
      <template #head>
        <TableRow>
          <TableCell th>ID</TableCell>
          <TableCell th>Class</TableCell>
          <TableCell th>Subclass</TableCell>
          <TableCell th>Vendor</TableCell>
          <TableCell th>Product</TableCell>
          <TableCell th>Driver</TableCell>
        </TableRow>
      </template>

      <template #body>
        <TableRow v-for="device in devices" :key="itemID(device)">
          <TableCell>{{ device.metadata.id }}</TableCell>
          <TableCell>{{ device.spec.class || '-' }}</TableCell>
          <TableCell>{{ device.spec.subclass || '-' }}</TableCell>
          <TableCell>{{ device.spec.vendor || '-' }}</TableCell>
          <TableCell>{{ device.spec.product || '-' }}</TableCell>
          <TableCell>{{ device.spec.driver || '-' }}</TableCell>
        </TableRow>
      </template>
    </TableRoot>
  </div>
</template>
