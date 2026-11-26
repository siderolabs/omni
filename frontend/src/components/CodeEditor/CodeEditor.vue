<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script lang="ts">
// The `monaco-editor` entry point registers every language, and with them their language workers (TypeScript alone
// is ~7 MB), which would all be emitted into the build. Only load the core editor, its features and YAML.
import 'monaco-editor/features/register.all'
import 'monaco-editor/languages/definitions/yaml/register'

import type * as MonacoFull from 'monaco-editor'
import * as monaco from 'monaco-editor/editor'
import { configureMonacoYaml, type JSONSchema, type SchemasSettings } from 'monaco-yaml'
import { coerce, compare, gt, lt, parse } from 'semver'

import { DefaultTalosVersion } from '@/api/resources'
import { createOmniMonacoTheme, OMNI_CODE_THEME } from '@/lib/code-theme'
import { getDocsLink, majorMinorVersion } from '@/methods'
import { useTheme } from '@/methods/theme'
import configSchemas from '@/schemas'

/**
 * Adds markers to the editor's model. Monaco is passed in so callers don't import it themselves, which would pull it into
 * their chunk and defeat the lazy loading in ./index.ts.
 */
export type CodeEditorValidator = (
  model: monaco.editor.ITextModel,
  tokens: monaco.Token[],
  api: typeof monaco,
) => monaco.editor.IMarkerData[]

// Monaco's internal workers (tokenization, folding, etc.) go through this path.
window.MonacoEnvironment = {
  getWorker(_, label) {
    if (label === 'yaml') {
      return new Worker(new URL('./yaml.worker.ts', import.meta.url), { type: 'module' })
    }

    return new Worker(new URL('./editor.worker.ts', import.meta.url), { type: 'module' })
  },
}

// Options monaco-worker-manager (used by monaco-yaml) still passes to monaco.editor.createWebWorker.
interface LegacyWebWorkerOptions {
  label?: string
  createData?: unknown
}

// Work around https://github.com/remcohaszing/monaco-yaml/issues/272.
// monaco.editor.createWebWorker only accepts an already-created worker now, so translate the legacy options the same
// way monaco's own (no longer exported) internal/common/workers.js createWebWorker does.
const { createWebWorker: oldCreateWebWorker } = monaco.editor
monaco.editor.createWebWorker = <T extends object>(
  opts: monaco.editor.IInternalWebWorkerOptions | LegacyWebWorkerOptions,
) => {
  if ('worker' in opts) {
    return oldCreateWebWorker<T>(opts)
  }

  const worker = window.MonacoEnvironment!.getWorker!(
    'workerMain.js',
    opts.label ?? 'monaco-editor-worker',
  )

  return oldCreateWebWorker<T>({
    worker: Promise.resolve(worker).then((w) => {
      // The first message boots the worker, the second carries the create data.
      w.postMessage('ignore')
      w.postMessage(opts.createData)
      return w
    }),
  })
}

const configSchemaLoaders = new Map(
  Object.entries(configSchemas).map(
    ([path, load]) =>
      [path.replace(/\.\/config_(.*)\.schema\.json/, '$1').replace('_', '.'), load] as const,
  ),
)

function withPatchDelete(node: boolean | JSONSchema, version: string): boolean | JSONSchema {
  if (typeof node !== 'object') return node

  if (node.properties) {
    node.properties.$patch = {
      type: 'string',
      title: '$patch',
      enum: ['delete'],
      description: `Delete the configuration block with a strategic merge delete patch.\nSee ${getDocsLink('talos', '/configure-your-talos-cluster/system-configuration/patching', { talosVersion: version })}`,
    }

    for (const key in node.properties) {
      if (key === '$patch') continue

      node.properties[key] = withPatchDelete(node.properties[key], version)
    }
  }

  // Map-shaped schemas (e.g. `taints`, `labels`) use `patternProperties` instead of
  // `properties`, so their value schemas need patching too.
  if (node.patternProperties) {
    for (const key in node.patternProperties) {
      node.patternProperties[key] = withPatchDelete(node.patternProperties[key], version)
    }
  }

  return node
}

/** The schema version to validate a Talos version's config against, clamped to the versions there are schemas for. */
function schemaVersionFor(talosVersion: string) {
  const versions = [...configSchemaLoaders.keys()]
    .map((version) => parse(`${version}.0`, false, true))
    .sort(compare)

  const minVersion = versions.at(0)
  const maxVersion = versions.at(-1)
  let version = coerce(talosVersion) ?? parse(DefaultTalosVersion, false, true)

  if (minVersion && lt(version, minVersion)) version = minVersion
  if (maxVersion && gt(version, maxVersion)) version = maxVersion

  return majorMinorVersion(version.format())
}

const schemaSettings = new Map<string, Promise<SchemasSettings>>()

/** Loads and prepares a version's schema the first time an editor uses it. */
function getSchemaSettings(version: string, load: (typeof configSchemas)[string]) {
  let settings = schemaSettings.get(version)

  if (!settings) {
    settings = load().then(
      (origSchema) => {
        const schema: typeof origSchema = JSON.parse(JSON.stringify(origSchema))

        for (const name in schema.$defs) {
          schema.$defs[name] = withPatchDelete(schema.$defs[name], version) as JSONSchema
        }

        return { uri: schema.$id!, fileMatch: [], schema } satisfies SchemasSettings
      },
      (error) => {
        // Let the next editor that needs it try again, e.g. after a network blip.
        schemaSettings.delete(version)
        throw error
      },
    )

    schemaSettings.set(version, settings)
  }

  return settings
}

/** Loads and prepares the schema an editor for this Talos version will use, before it is shown. */
export async function preloadConfigSchema(talosVersion = DefaultTalosVersion) {
  const version = schemaVersionFor(talosVersion)
  const load = configSchemaLoaders.get(version)

  if (load) await getSchemaSettings(version, load)
}

// monaco-yaml is typed against the full `monaco-editor` module, but only uses `editor` and `languages`.
const monacoYaml = configureMonacoYaml(monaco as typeof MonacoFull, {
  hover: true,
  completion: true,
  validate: true,
  format: {
    enable: true,
  },
  schemas: [],
})

const modelSchemas = new Map<string, string>()
let schemaUpdate = 0

async function setModelSchema(modelId: string, version: string | null) {
  if (typeof version === 'string') {
    modelSchemas.set(modelId, version)
  } else {
    modelSchemas.delete(modelId)
  }

  // Only register the versions some editor is using, so the rest are never downloaded.
  const update = ++schemaUpdate
  const versions = [...new Set(modelSchemas.values())].flatMap((version) => {
    const load = configSchemaLoaders.get(version)

    return load ? [{ version, load }] : []
  })

  const settings = await Promise.all(
    versions.map(({ version, load }) => getSchemaSettings(version, load)),
  )

  // A later call has the newer assignments, so let it apply them.
  if (update !== schemaUpdate) return

  await monacoYaml.update({
    ...monacoYaml.getOptions(),
    schemas: settings.map((setting, i) => ({
      ...setting,
      fileMatch: [...modelSchemas]
        .filter(([, assigned]) => assigned === versions[i].version)
        .map(([id]) => `*${id}.yaml`),
    })),
  })
}

// Can't use CSS variables inside monaco https://github.com/microsoft/monaco-editor/issues/2427
const styles = getComputedStyle(document.documentElement)

const { theme } = useTheme()

// Monaco themes are global, so one watcher re-themes every open editor. Post
// flush reads the palette after the page has switched.
watch(
  theme,
  (value) => {
    monaco.editor.defineTheme(OMNI_CODE_THEME, createOmniMonacoTheme(value))
    monaco.editor.setTheme(OMNI_CODE_THEME)
  },
  { immediate: true, flush: 'post' },
)
</script>

<script setup lang="ts">
import { computed, onWatcherCleanup, ref, useId, useTemplateRef, watch } from 'vue'

type Props = {
  options?: monaco.editor.IEditorOptions & monaco.editor.IGlobalEditorOptions
  validators?: CodeEditorValidator[]
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
const schemaVersion = computed(() => schemaVersionFor(talosVersion))

const wordWrap = ref<'on' | 'off'>('off')

const editorOptions = computed<monaco.editor.IEditorOptions & monaco.editor.IGlobalEditorOptions>(
  () => {
    const rootFontSize = parseFloat(styles.fontSize)

    return {
      theme: OMNI_CODE_THEME,
      wordWrap: wordWrap.value,
      fontSize: parseFloat(styles.getPropertyValue('--text-sm')) * rootFontSize,
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
    }
  },
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

  const action = instance.addAction({
    id: 'toggle-word-wrap',
    label: 'Toggle Word Wrap',
    keybindings: [monaco.KeyMod.Alt | monaco.KeyCode.KeyZ],
    contextMenuGroupId: 'view',
    contextMenuOrder: 1,
    run() {
      wordWrap.value = wordWrap.value === 'on' ? 'off' : 'on'
    },
  })

  function validate() {
    if (!validators?.length) return

    const tokens = monaco.editor.tokenize(model.getValue(), 'yaml')
    const markers = validators.flatMap((validator) => validator(model, tokens.flat(), monaco))

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
    action.dispose()
    instance.dispose()

    instanceRef = undefined
  })
})

watch(
  [schemaVersion, () => disableConfigValidation],
  ([version, disableConfigValidation]) => {
    setModelSchema(modelId, disableConfigValidation ? null : version)

    onWatcherCleanup(() => {
      setModelSchema(modelId, null)
    })
  },
  { immediate: true },
)

// updateOptions is a partial merge, you must explicitly toggle options
// simply deleting a key from options won't reset it to its default value
watch(editorOptions, (options) => instanceRef?.updateOptions(options))
</script>

<template>
  <!--
    With fixedOverflowWidgets, Monaco mounts the context menu's shadow root on this element, beside .monaco-editor rather
    than inside it. Monaco only defines its theme variables (--vscode-menu-background etc.) on .monaco-editor and
    .monaco-component, so without this class the menu has no colours and renders transparent.
  -->
  <div ref="editor" class="monaco-component"></div>
</template>
