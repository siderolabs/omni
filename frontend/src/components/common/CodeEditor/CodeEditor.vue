<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div id="editor" class="w-full h-full" ref="editor"/>
</template>

<script setup lang="ts">
import configSchema from "@/schemas/config.schema.json";
import { naturals } from "@/vars/colors";
import { ref, toRefs, watch } from "vue";

import * as monaco from "monaco-editor";
import { configureMonacoYaml } from "monaco-yaml";

type Props = {
  value: string,
  editorDidMount?: (editor: monaco.editor.IStandaloneCodeEditor) => void,
  options?: monaco.editor.IStandaloneEditorConstructionOptions,
  validators?: ((model: monaco.editor.ITextModel, tokens: monaco.Token[]) => monaco.editor.IMarkerData[])[]
};

const emit = defineEmits(["update:value"]);
const props = defineProps<Props>();

const { value } = toRefs(props);

const editor = ref<HTMLElement>();

let instance: monaco.editor.IStandaloneCodeEditor | undefined;

if (!window['monacoConfigured']) {
  window['monacoConfigured'] = true;

  configureMonacoYaml(monaco, {
    hover: true,
    completion: true,
    validate: true,
    format: true,
    schemas: [
      {
        uri: configSchema.$id,
        fileMatch: ["*"],
        schema: configSchema,
      },
    ],
  });
}

monaco.editor.defineTheme("sidero", {
  base: 'vs-dark',
  inherit: true,
  rules: [],
  colors: {
    "dropdown.background": naturals.N3,

    "editor.background": "#00000000",

    "editorHoverWidget.background": naturals.N3,
    "editorHoverWidget.border": naturals.N7,

    "editorOverviewRuler.border": "#00000000",

    "editorWidget.background": naturals.N3,
    "editorWidget.border": naturals.N7,

    "input.background": naturals.N1,
    "input.border": naturals.N7,
  }
});

watch(value, (val: string) => {
  const model = instance?.getModel();

  if (!model) {
    return;
  }

  if (val === model.getValue()) {
    return;
  }

  model.setValue(val);
});

watch(editor, () => {
  if (!editor.value) {
    return;
  }

  instance = monaco.editor.create(editor.value, {
    value: [
      value.value
    ].join('\n'),
    language: 'yaml',
    theme: 'sidero',
    fontSize: 14,
    fontFamily: '"Roboto Mono", monospace',
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
    ...props.options
  });

  if (props.editorDidMount) {
    props.editorDidMount(instance);
  }
});

monaco.editor.onDidCreateModel(function(model) {
  let validate: () => void;

  if (props.validators?.length) {
    validate = () => {
      var textToValidate = model.getValue();

      let markers: monaco.editor.IMarkerData[] = [];
      const tokens = monaco.editor.tokenize(textToValidate, "yaml");

      for (const validator of props.validators!) {
        markers = markers.concat(validator(model, tokens.flat()));
      }

      monaco.editor.setModelMarkers(model, "sanityCheck", markers);
    }

    validate();
  }

  let handle: any;

  model.onDidChangeContent(() => {
    // debounce
    clearTimeout(handle);

    emit('update:value', model.getValue());

    if (validate) {
      handle = setTimeout(() => validate(), 500);
    }
  });
});

monaco.editor.defineTheme("sidero", {
  base: 'vs-dark',
  inherit: true,
  rules: [],
  colors: {
    "dropdown.background": naturals.N3,

    "editor.background": "#00000000",

    "editorHoverWidget.background": naturals.N3,
    "editorHoverWidget.border": naturals.N7,

    "editorOverviewRuler.border": "#00000000",

    "editorWidget.background": naturals.N3,
    "editorWidget.border": naturals.N7,

    "input.background": naturals.N1,
    "input.border": naturals.N7,
  }
});
</script>

<style>
.editor h4 {
  @apply font-bold;
}
</style>
