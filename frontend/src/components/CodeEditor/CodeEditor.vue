<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script lang="ts">
import * as monaco from 'monaco-editor'
import { configureMonacoYaml, type SchemasSettings } from 'monaco-yaml'

import { getDocsLink } from '@/methods'
import configSchemas from '@/schemas'

// Monaco's internal workers (tokenization, folding, etc.) go through this path.
window.MonacoEnvironment = {
  getWorker(_, label) {
    if (label === 'yaml') {
      return new Worker(new URL('./yaml.worker.ts', import.meta.url), { type: 'module' })
    }

    return new Worker(new URL('./editor.worker.ts', import.meta.url), { type: 'module' })
  },
}

// Work around https://github.com/remcohaszing/monaco-yaml/issues/272.
const { createWebWorker: oldCreateWebWorker } = monaco.editor
monaco.editor.createWebWorker = (
  opts: monaco.IWebWorkerOptions | monaco.editor.IInternalWebWorkerOptions,
) => {
  if ('worker' in opts) {
    return oldCreateWebWorker(opts)
  }

  return monaco.createWebWorker(opts)
}

const configSchemaMap = Object.entries(configSchemas).map(
  ([path, schema]) =>
    [path.replace(/\.\/config_(.*)\.schema\.json/, '$1').replace('_', '.'), schema] as const,
)

const versionedSchemas = configSchemaMap.map(([version, origSchema]) => {
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
    version,
    settings: {
      uri: schema.$id!,
      fileMatch: [],
      schema,
    } satisfies SchemasSettings,
  }
})

const monacoYaml = configureMonacoYaml(monaco, {
  hover: true,
  completion: true,
  validate: true,
  format: {
    enable: true,
  },
  schemas: [],
})

const modelSchemas = new Map<string, string>()

function refreshSchemas() {
  return monacoYaml.update({
    ...monacoYaml.getOptions(),
    schemas: versionedSchemas.map(({ version, settings }) => ({
      ...settings,
      fileMatch: [...modelSchemas]
        .filter(([, assigned]) => assigned === version)
        .map(([modelId]) => `*${modelId}.yaml`),
    })),
  })
}

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

    'editor.background': styles.getPropertyValue('--color-naturals-n0'),

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
import { coerce, compare, gt, lt, parse } from 'semver'
import { computed, onWatcherCleanup, useId, useTemplateRef, watch } from 'vue'

import { DefaultTalosVersion } from '@/api/resources'
import { majorMinorVersion } from '@/methods'

type Props = {
  options?: monaco.editor.IEditorOptions & monaco.editor.IGlobalEditorOptions
  validators?: ((
    model: monaco.editor.ITextModel,
    tokens: monaco.Token[],
  ) => monaco.editor.IMarkerData[])[]
  talosVersion?: string
  disableConfigValidation?: boolean
}

const {
  options,
  validators,
  disableConfigValidation,
  talosVersion = DefaultTalosVersion,
} = defineProps<Props>()

const modelValue = defineModel<string>({ default: '' })

const editor = useTemplateRef<HTMLDivElement>('editor')

let instanceRef: monaco.editor.IStandaloneCodeEditor | undefined

watch(modelValue, (val) => {
  const model = instanceRef?.getModel()
  if (!model) return

  if (val !== model.getValue()) {
    model.setValue(val)
  }
})

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

const editorOptions = computed<monaco.editor.IEditorOptions & monaco.editor.IGlobalEditorOptions>(
  () => ({
    theme: SIDERO_THEME,
    fontSize: 14,
    fontFamily: styles.getPropertyValue('--font-mono'),
    automaticLayout: true,
    tabSize: 2,
    insertSpaces: true,
    detectIndentation: false,
    trimAutoWhitespace: true,
    renderWhitespace: 'boundary',
    fixedOverflowWidgets: true,
    lineNumbersMinChars: 3,
    lineDecorationsWidth: 5,
    stickyScroll: {
      enabled: true,
    },
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
  }),
)

watch(editor, (editor) => {
  if (!editor) return

  const model = monaco.editor.createModel(
    modelValue.value,
    'yaml',
    monaco.Uri.parse(`inmemory://${modelId}.yaml`),
  )

  const instance = monaco.editor.create(editor, {
    model,
    ...editorOptions.value,
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

    modelValue.value = model.getValue()

    handle = window.setTimeout(validate, 500)
  })

  instanceRef = instance

  onWatcherCleanup(() => {
    clearTimeout(handle)

    contentChangeListener.dispose()
    model.dispose()
    instance.dispose()

    instanceRef = undefined
  })
})

watch(
  [schemaVersion, () => disableConfigValidation],
  ([version, disableConfigValidation]) => {
    if (disableConfigValidation) {
      modelSchemas.delete(modelId)
    } else {
      modelSchemas.set(modelId, majorMinorVersion(version.format()))
    }

    refreshSchemas()

    onWatcherCleanup(() => {
      modelSchemas.delete(modelId)
      refreshSchemas()
    })
  },
  { immediate: true },
)

// updateOptions is a partial merge, you must explicitly toggle options
// simply deleting a key from options won't reset it to its default value
watch(editorOptions, (options) => instanceRef?.updateOptions(options))
</script>

<template>
  <div ref="editor"></div>
</template>
