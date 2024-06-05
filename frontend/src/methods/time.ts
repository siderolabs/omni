// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

import { DateTime, Duration, DurationLikeObject } from "luxon"

export const relativeISO = (input: string): string => {
  return parseISO(input).toRelative()!;
}

export const formatISO = (input: string, format?: string): string => {
  if (!format) {
    format = "dd/MM/yyyy HH:mm:ss";
  }

  return parseISO(input).toFormat(format);
}

export const isoNow = (): string => {
  const date = DateTime.now();

  return date.setLocale("en-US").toUTC().toISO();
}

export const parseDuration = (input: string): Duration => {
  const units: Record<"h" | "m" | "s", keyof DurationLikeObject> = {
    "h": "hours",
    "m": "minutes",
    "s": "seconds",
  };

  let buffer = "";

  const duration: DurationLikeObject = {};

  for (const s of input) {
    if (units[s]) {
      if (duration[units[s]] !== undefined) {
        throw new Error(`failed to parse duration ${input}`);
      }

      const value = +buffer;

      if (buffer === null || buffer === '' || isNaN(value)) {
        throw new Error(`failed to parse duration ${input}`);
      }

      buffer = "";

      duration[units[s]] = value;

      continue;
    }

    buffer += s;
  }

  if (buffer !== "") {
    throw new Error(`failed to parse duration ${input}`);
  }

  return Duration.fromDurationLike(duration);
}

export const formatDuration = (duration: Duration): string => {
  let res = "";

  duration = duration.shiftTo("seconds");

  for (const units of [["seconds", "s"]]) {
    const key = units[0];
    const suffix = units[1];

    const value = duration.get(key as keyof DurationLikeObject);
    if (value > 0) {
      res += `${value}${suffix}`;
    }
  }

  if (res === "") {
    res = "0";
  }

  return res;
};

const parseISO = (input: string): DateTime => {
  return DateTime.fromISO(input).setLocale("en-US").toLocal();
}
