<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { useMounted } from '@vueuse/core'
import { compare, major, minor } from 'semver'
import { computed, onBeforeMount, useId } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import type { TalosVersionSpec } from '@/api/omni/specs/omni.pb'
import type { QuirksSpec } from '@/api/omni/specs/virtual.pb'
import {
  DefaultNamespace,
  DefaultTalosVersion,
  QuirksType,
  TalosVersionType,
  VirtualNamespace,
} from '@/api/resources'
import FormLabel from '@/components/FormLabel/FormLabel.vue'
import TIcon from '@/components/Icon/TIcon.vue'
import SelectContent from '@/components/SelectList/components/SelectContent.vue'
import SelectGroup from '@/components/SelectList/components/SelectGroup.vue'
import SelectGroupLabel from '@/components/SelectList/components/SelectGroupLabel.vue'
import SelectIcon from '@/components/SelectList/components/SelectIcon.vue'
import SelectItemType from '@/components/SelectList/components/SelectItem.vue'
import SelectItemIndicator from '@/components/SelectList/components/SelectItemIndicator.vue'
import SelectItemText from '@/components/SelectList/components/SelectItemText.vue'
import SelectLabel from '@/components/SelectList/components/SelectLabel.vue'
import SelectPortal from '@/components/SelectList/components/SelectPortal.vue'
import SelectRoot from '@/components/SelectList/components/SelectRoot.vue'
import SelectScrollDownButton from '@/components/SelectList/components/SelectScrollDownButton.vue'
import SelectScrollUpButton from '@/components/SelectList/components/SelectScrollUpButton.vue'
import SelectTrigger from '@/components/SelectList/components/SelectTrigger.vue'
import SelectValue from '@/components/SelectList/components/SelectValue.vue'
import SelectViewport from '@/components/SelectList/components/SelectViewport.vue'
import Tooltip from '@/components/Tooltip/Tooltip.vue'
import { useResourceList } from '@/methods/useResourceList'
import { AUTOMATIC_VERSION } from '@/views/InstallationMedia/useFormState'

const { defaultValue, requiresTalosctlSupport, includeAutomatic } = defineProps<{
  title?: string
  overheadTitle?: boolean
  defaultValue?: string
  requiresTalosctlSupport?: boolean
  includeAutomatic?: boolean
}>()

const emit = defineEmits<{
  checkedValue: [value: string]
}>()

const triggerId = useId()
const isMounted = useMounted()

const selectedItem = defineModel<string>({
  set(v) {
    if (isMounted.value && v !== selectedItem.value) emit('checkedValue', v)

    return v
  },
})

const { data: talosVersionList, loading: talosVersionsLoading } = useResourceList<TalosVersionSpec>(
  {
    runtime: Runtime.Omni,
    resource: {
      type: TalosVersionType,
      namespace: DefaultNamespace,
    },
  },
)

const { data: quirks, loading: quirksLoading } = useResourceList<QuirksSpec>(() => ({
  skip: !requiresTalosctlSupport,
  runtime: Runtime.Omni,
  resource: {
    type: QuirksType,
    namespace: VirtualNamespace,
  },
}))

const versionMap = computed(() =>
  Object.fromEntries(talosVersionList.value.map((v) => [v.spec.version!, v] as const)),
)

// Passing this as a default option to defineModel doesn't work due to the macro's limitations
onBeforeMount(() => {
  if (typeof defaultValue !== 'undefined' && typeof selectedItem.value === 'undefined') {
    selectedItem.value = defaultValue
  }
})

interface SelectItemType {
  label: string
  value: string
  disabled?: boolean
  tooltip?: string
}

const selectItems = computed(() => {
  const versions: SelectItemType[] = []

  if (includeAutomatic) {
    versions.push({ label: 'Automatic', value: AUTOMATIC_VERSION })
  }

  return versions.concat(
    talosVersionList.value
      .filter((v) => {
        if (v.spec.deprecated) return false
        if (!requiresTalosctlSupport) return true

        const quirk = quirks.value.find((q) => q.metadata.id === v.spec.version)

        return quirk?.spec.supports_factory_talosctl
      })
      .map(({ spec: { version, unsupported } }) => ({
        label: version!,
        value: version!,
        disabled: unsupported,
        tooltip: unsupported ? `This Omni release does not support Talos ${version}.` : undefined,
      }))
      .sort((a, b) => compare(b.value, a.value)),
  )
})

interface SelectGroupType {
  label?: string
  items: SelectItemType[]
}

const selectGroups = computed(() => {
  const groups: SelectGroupType[] = []

  for (const item of selectItems.value) {
    // Items without a real version (e.g. Automatic) stay in their own unlabelled group
    const label =
      item.value === AUTOMATIC_VERSION ? undefined : `${major(item.value)}.${minor(item.value)}`
    const last = groups.at(-1)

    if (last && last.label === label && typeof label !== 'undefined') {
      last.items.push(item)
    } else {
      groups.push({ label, items: [item] })
    }
  }

  return groups
})

function labelForItem(item?: string) {
  return selectItems.value.find((i) => i.value === item)?.label ?? ''
}

function isItemEnterprise(item?: string) {
  if (item === AUTOMATIC_VERSION) {
    item = DefaultTalosVersion
  }

  if (!item) return false

  return versionMap.value[item]?.spec.is_enterprise ?? false
}
</script>

<template>
  <component :is="title && overheadTitle ? 'label' : 'div'" class="inline-block">
    <FormLabel v-if="title && overheadTitle" as="span" class="mb-4">
      {{ title }}
    </FormLabel>

    <SelectRoot v-model="selectedItem" :disabled="talosVersionsLoading || quirksLoading">
      <SelectTrigger :id="triggerId">
        <SelectValue>
          <SelectLabel v-if="title && !overheadTitle" :for="triggerId" aria-hidden="true">
            {{ title }}
          </SelectLabel>

          <span class="inline-flex items-center gap-2">
            {{ labelForItem(selectedItem) }}

            <span
              v-if="isItemEnterprise(selectedItem)"
              class="resource-label label-violet text-[0.625rem]"
            >
              enterprise
            </span>
          </span>
        </SelectValue>
        <SelectIcon>
          <TIcon class="size-4 fill-current transition-all duration-300" icon="chevron-down" />
        </SelectIcon>
      </SelectTrigger>

      <SelectPortal>
        <SelectContent>
          <SelectScrollUpButton>
            <TIcon icon="chevron-up" class="mx-auto size-(--arrow-size)" />
          </SelectScrollUpButton>

          <SelectViewport>
            <SelectGroup
              v-for="(group, index) in selectGroups"
              :key="group.label ?? index"
              class="not-first:mt-2"
            >
              <SelectGroupLabel v-if="group.label">
                {{ group.label }}
              </SelectGroupLabel>

              <Tooltip
                v-for="item in group.items"
                :key="item.value"
                :description="item.tooltip"
                :disabled="!item.tooltip"
                placement="right"
              >
                <SelectItemType :value="item.value" :disabled="item.disabled">
                  <span class="size-3">
                    <SelectItemIndicator as-child>
                      <TIcon icon="check" class="size-full" />
                    </SelectItemIndicator>
                  </span>

                  <SelectItemText class="flex grow items-center justify-between gap-2">
                    {{ item.label }}

                    <span
                      v-if="isItemEnterprise(item.value)"
                      class="resource-label label-violet text-[0.625rem] font-normal"
                    >
                      enterprise
                    </span>
                  </SelectItemText>
                </SelectItemType>
              </Tooltip>
            </SelectGroup>
          </SelectViewport>

          <SelectScrollDownButton>
            <TIcon icon="chevron-down" class="mx-auto size-(--arrow-size)" />
          </SelectScrollDownButton>
        </SelectContent>
      </SelectPortal>
    </SelectRoot>
  </component>
</template>
