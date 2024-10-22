// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

import { Data } from "@/api/common/common.pb";
import { subscribe, StreamingRequest, Stream } from "@/api/grpc";
import { isRef, onMounted, onUnmounted, ref, Ref, ComputedRef, watch } from "vue";
import { fetchOption } from "@/api/fetch.pb";

export type LogLine = {
  date?: string
  msg: string
}

export interface LogParser {
  parse: (chunk: string) => LogLine[]
  reset: () => void
}

export class DefaultLogParser {
  private parseLine: (chunk: string) => LogLine

  constructor(parseLine: (chunk: string) => LogLine) {
    this.parseLine = parseLine;
  }

  parse(chunk: string): LogLine[] {
    return [
      this.parseLine(chunk)
    ];
  }

  reset() {
    //noop
  }
}

export class LineDelimitedLogParser {
  private parseLine: (chunk: string) => LogLine
  private buffer: string = "";

  constructor(parseLine: (chunk: string) => LogLine) {
    this.parseLine = parseLine;
  }

  public setLineParser(parseLine: (chunk: string) => LogLine) {
    this.parseLine = parseLine;
  }

  parse(chunk: string): LogLine[] {
    this.buffer += chunk;
    const splitPoint = this.buffer.lastIndexOf("\n");

    if (splitPoint === -1) {
      return [];
    }

    const logs: LogLine[] = [];
    for (const l of this.buffer.slice(0, splitPoint).split("\n")) {
      if (!l.trim()) {
        continue;
      }

      logs.push(this.parseLine(l));
    }

    this.buffer = this.buffer.slice(splitPoint+1, this.buffer.length);

    return logs;
  }

  reset() {
    this.buffer = "";
  }
}

export const setupLogStream = <R extends Data, T>(logs: Ref<LogLine[]>, method: StreamingRequest<R, T>, params: T | ComputedRef<T> | Ref<T>, logParser: LogParser = new DefaultLogParser((l: string): LogLine => { return {msg: l} }), ...options: fetchOption[]): Ref<Stream<R, T> | undefined> => {
  const stream: Ref<Stream<R, T> | undefined> = ref();
  let buffer: LogLine[] = [];
  let flush: NodeJS.Timeout | undefined;

  const reset = () => {
    logs.value = [];
    logParser.reset();
    clearTimeout(flush);
  };

  const init = () => {
    if (stream.value) {
      stream.value.shutdown();
    }

    reset();

    let clearLogs = false;

    const p = isRef(params) ? params.value : params;

    stream.value = subscribe(
      method,
      p,
      (resp: Data & { error?: string }) => {
        clearTimeout(flush);

        if (resp.error) {
          clearLogs = true;
          return;
        }

        if (clearLogs) reset();

        clearLogs = false;

        if (resp.bytes) {
          const line = window.atob(resp.bytes.toString());

          try {
            buffer.push(...logParser.parse(line));
          } catch (e) {
            console.error(`failed to parse line ${line}`, e);
          }
        }

        // accumulate frequent updates and then flush them in a single call
        flush = setTimeout(() => {
          logs.value = logs.value.concat(buffer);
          buffer = [];
        }, 50);
      },
      options,
    );
  };

  onMounted(init);

  onUnmounted(() => {
    if (stream.value) stream.value.shutdown();
  });

  if (isRef(params)) {
    watch(params, init)
  }

  return stream;
}
