<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <MonacoEditor
      :options="finalOptions"
      @update:value="(val) => emit('update:value', val)"
      :value="value"
      @editorWillMount="editorWillMount"
      @editorDidMount="editorDidMount"
      width="100%"
      height="100%"
  ></MonacoEditor>
</template>

<script setup lang="ts">
import MonacoEditor from "monaco-editor-vue3";
import { setDiagnosticsOptions } from "monaco-yaml";
import configSchema from "@/schemas/config.schema.json";
import * as monaco from 'monaco-editor/esm/vs/editor/editor.api';
import { naturals } from "@/vars/colors";

type Props = {
  value: string,
  editorDidMount?: (editor: monaco.editor.IStandaloneCodeEditor) => void,
  options?: monaco.editor.IStandaloneEditorConstructionOptions,
  validators?: ((model: monaco.editor.ITextModel, tokens: monaco.Token[]) => monaco.editor.IMarkerData[])[]
};

const emit = defineEmits(["update:value"]);
const props = defineProps<Props>();

const editorWillMount = () => {
  setDiagnosticsOptions({
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
  })
};

if (props.validators?.length) {
  monaco.editor.onDidCreateModel(function(model) {
      function validate() {
        var textToValidate = model.getValue();

        let markers: monaco.editor.IMarkerData[] = [];
        const tokens = monaco.editor.tokenize(textToValidate, "yaml");

        for (const validator of props.validators!) {
          markers = markers.concat(validator(model, tokens.flat()));
        }

        monaco.editor.setModelMarkers(model, "sanityCheck", markers);
      }

      let handle: any;

      model.onDidChangeContent(() => {
        // debounce
        clearTimeout(handle);
        handle = setTimeout(() => validate(), 500);
      });

      validate();
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

const baseOptions: monaco.editor.IStandaloneEditorConstructionOptions = {
  theme: "sidero",
  language: "yaml",
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
};

const finalOptions = {
  ...baseOptions,
  ...props.options,
}

</script>

<style>
.monaco-editor-vue3 h4 {
  @apply font-bold;
}
</style>
