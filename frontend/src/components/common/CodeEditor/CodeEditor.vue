<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script lang="ts">
import configSchemas from '@/schemas'

const configSchemaMap = Object.entries(configSchemas).map(
  ([path, schema]) =>
    [path.replace(/\.\/config_(.*)\.schema\.json/, '$1').replace('_', '.'), schema] as const,
)

const schemas = configSchemaMap.map<SchemasSettings>(([version, origSchema]) => {
  const schema: typeof origSchema = JSON.parse(JSON.stringify(origSchema))

  for (const name in schema.$defs) {
    const def = schema.$defs[name as keyof typeof schema.$defs]

    if (def.properties) {
      def.properties.$patch = {
        type: 'string',
        title: '$patch',
        enum: ['delete'],
        description: `Delete the configuration block with a strategic merge delete patch.\nSee ${getDocsLink('talos', '/configure-your-talos-cluster/system-configuration/patching', { talosVersion: version })}`,
      }
    }
  }

  return {
    uri: schema.$id!,
    fileMatch: [`*_${version}.yaml`],
    schema,
  }
})

configureMonacoYaml(monaco, {
  hover: true,
  completion: true,
  validate: true,
  format: true,
  schemas,
})

// Can't use CSS variables inside monaco https://github.com/microsoft/monaco-editor/issues/2427
const styles = getComputedStyle(document.documentElement)
const SIDERO_THEME = 'sidero'

monaco.editor.defineTheme(SIDERO_THEME, {
  base: 'vs-dark',
  inherit: true,
  rules: [],
  colors: {
    'dropdown.background': styles.getPropertyValue('--color-naturals-n3'),

    'editorStickyScroll.background': styles.getPropertyValue('--color-naturals-n0'),

    'editor.background': '#00000000',

    'editorHoverWidget.background': styles.getPropertyValue('--color-naturals-n3'),
    'editorHoverWidget.border': styles.getPropertyValue('--color-naturals-n7'),

    'editorOverviewRuler.border': '#00000000',

    'editorWidget.background': styles.getPropertyValue('--color-naturals-n3'),
    'editorWidget.border': styles.getPropertyValue('--color-naturals-n7'),

    'input.background': styles.getPropertyValue('--color-naturals-n1'),
    'input.border': styles.getPropertyValue('--color-naturals-n7'),
  },
})
</script>

<script setup lang="ts">
import * as monaco from 'monaco-editor'
import { configureMonacoYaml, type SchemasSettings } from 'monaco-yaml'
import { coerce, compare, gt, lt, parse } from 'semver'
import { computed, onWatcherCleanup, useId, useTemplateRef, watch } from 'vue'

import { DefaultTalosVersion } from '@/api/resources'
import { getDocsLink, majorMinorVersion } from '@/methods'

type Props = {
  value: string
  editorDidMount?: (editor: monaco.editor.IStandaloneCodeEditor) => void
  options?: monaco.editor.IStandaloneEditorConstructionOptions
  validators?: ((
    model: monaco.editor.ITextModel,
    tokens: monaco.Token[],
  ) => monaco.editor.IMarkerData[])[]
  talosVersion?: string
}

const emit = defineEmits<{
  'update:value': [string]
  editorDidMount: [monaco.editor.IStandaloneCodeEditor]
}>()

const { value, options, validators, talosVersion = DefaultTalosVersion } = defineProps<Props>()

const editor = useTemplateRef<HTMLDivElement>('editor')

let instanceRef: monaco.editor.IStandaloneCodeEditor | undefined

watch(
  () => value,
  (val) => {
    const model = instanceRef?.getModel()

    if (!model) {
      return
    }

    if (val === model.getValue()) {
      return
    }

    model.setValue(val)
  },
)

const modelId = useId()
const schemaVersion = computed(() => {
  const versions = configSchemaMap
    .map(([version]) => parse(`${version}.0`, false, true))
    .sort(compare)

  const minVersion = versions.at(0)
  const maxVersion = versions.at(-1)
  const version = coerce(talosVersion)

  if (!version) return parse(DefaultTalosVersion, false, true)

  if (minVersion && lt(version, minVersion)) return minVersion
  if (maxVersion && gt(version, maxVersion)) return maxVersion

  return version
})

watch([editor, schemaVersion], () => {
  if (!editor.value) {
    return
  }

  const model = monaco.editor.createModel(
    value,
    'yaml',
    monaco.Uri.parse(
      `inmemory://${modelId}_${majorMinorVersion(schemaVersion.value.format())}.yaml`,
    ),
  )

  const instance = monaco.editor.create(editor.value, {
    model,
    theme: SIDERO_THEME,
    fontSize: 14,
    fontFamily: styles.getPropertyValue('--font-mono'),
    automaticLayout: true,
    tabSize: 2,
    fixedOverflowWidgets: true,
    minimap: {
      enabled: false,
    },
    inlineSuggest: {
      enabled: true,
    },
    quickSuggestions: {
      strings: true,
    },
    ...options,
  })

  function validate() {
    if (!validators?.length) return

    const tokens = monaco.editor.tokenize(model.getValue(), 'yaml')
    const markers = validators.flatMap((validator) => validator(model, tokens.flat()))

    monaco.editor.setModelMarkers(model, 'sanityCheck', markers)
  }

  validate()

  let handle: number | undefined

  const contentChangeListener = model.onDidChangeContent(() => {
    // debounce
    clearTimeout(handle)

    emit('update:value', model.getValue())

    handle = window.setTimeout(validate, 500)
  })

  emit('editorDidMount', instance)
  instanceRef = instance

  onWatcherCleanup(() => {
    clearTimeout(handle)

    contentChangeListener.dispose()
    model.dispose()
    instance.dispose()

    instanceRef = undefined
  })
})
</script>

<template>
  <div id="editor" ref="editor" class="h-full w-full" />
</template>

<style>
@reference "../../../index.css";

.editor h4 {
  @apply font-bold;
}

.monaco-editor {
  outline: 0;
}
</style>
