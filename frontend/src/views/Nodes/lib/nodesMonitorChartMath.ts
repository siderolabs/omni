// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

/**
 * Geometry for {@link NodesMonitorChart}. Everything here is numbers in,
 * numbers out, so it can be exercised without mounting anything.
 */
import { scaleLinear, scaleTime } from 'd3-scale'
import { area as d3Area, curveMonotoneX, line as d3Line } from 'd3-shape'
import { format as formatDate, milliseconds } from 'date-fns'

/** One update, holding every series' value for that moment. */
export interface Sample {
  t: number
  values: number[]
}

export interface Band {
  t: number
  y0: number
  y1: number
}

export interface ValueTick {
  value: number
  label: string
  y: number
}

export interface TimeTick {
  key: number
  label: string
  x: number
  anchor: 'start' | 'middle' | 'end'
}

export const MARGIN_TOP = 16
export const MARGIN_RIGHT = 8
export const MARGIN_BOTTOM = 24
export const Y_TICK_COUNT = 4

// Labels render at 0.625rem; roughly 6px per character is enough to reserve a
// gutter that fits the widest tick without measuring text.
const CHAR_WIDTH = 6
const GUTTER = 8
const CURSOR_GAP = 12
// Half an `HH:mm:ss` label, so a centred one never spills past the plot.
const LABEL_REACH = 24

/** A fixed bound, or one read from each update. */
export type Bound<T> = number | ((sample: T) => number)

/** Resolves a bound, which needs a sample only when it is a function. */
export function resolveBound<T>(value: Bound<T> | undefined, sample: T | undefined) {
  if (typeof value !== 'function') return value
  if (sample === undefined) return undefined

  return (value as (sample: T) => number)(sample)
}

/** At most one decimal, dropping a trailing `.0`. */
export function formatValue(value: number, format?: (value: number) => string) {
  return format ? format(value) : value.toFixed(1).replace(/\.0$/, '')
}

/** Appends an update, keeping only the most recent `tailEvents` of them. */
export function appendSample(samples: Sample[], sample: Sample, tailEvents: number): Sample[] {
  return [...samples, sample].slice(-tailEvents)
}

export function yDomain(
  samples: Sample[],
  stacked: boolean | undefined,
  min: number | undefined,
  max: number | undefined,
): [lo: number, hi: number] {
  const lo = min ?? 0
  const hi =
    max ??
    samples.reduce((prev, { values }) => {
      const top = stacked ? values.reduce((sum, v) => sum + v, 0) : Math.max(...values)

      return Math.max(prev, top)
    }, lo)

  // A zero-height domain makes the scale degenerate.
  return [lo, hi > lo ? hi : lo + 1]
}

export function yScale(domain: [number, number], height: number, autoMax: boolean) {
  const scale = scaleLinear()
    .domain(domain)
    .range([Math.max(height - MARGIN_BOTTOM, 0), MARGIN_TOP])

  // Without an explicit maximum, round the top up to the next tick so the peak
  // has headroom rather than touching the frame.
  return autoMax ? scale.nice(Y_TICK_COUNT) : scale
}

export function yTickValues(
  scale: ReturnType<typeof yScale>,
  domain: [number, number],
  boundsAreExplicit: boolean,
) {
  const [lo, hi] = domain

  // With an explicit min and max, split the range evenly. The formatter often
  // rescales the value (bytes to GiB, say), and evenly spaced ticks land on
  // round numbers in those units where d3's round raw numbers would not.
  if (boundsAreExplicit) {
    return Array.from({ length: Y_TICK_COUNT + 1 }, (_, i) => lo + ((hi - lo) * i) / Y_TICK_COUNT)
  }

  return scale.ticks(Y_TICK_COUNT)
}

export function yTicks(
  values: number[],
  scale: ReturnType<typeof yScale>,
  format?: (value: number) => string,
): ValueTick[] {
  return values.map((value) => ({
    value,
    label: formatValue(value, format),
    y: scale(value),
  }))
}

/** Gutter wide enough for the longest tick label. */
export function marginLeft(labels: string[]) {
  return Math.max(...labels.map((label) => label.length), 1) * CHAR_WIDTH + GUTTER
}

export function xDomain(samples: Sample[]): [lo: number, hi: number] {
  if (!samples.length) return [0, 1]

  const lo = samples[0].t
  const hi = samples[samples.length - 1].t

  return [lo, hi > lo ? hi : lo + 1]
}

export function xScale(domain: [number, number], width: number, left: number) {
  return scaleTime()
    .domain(domain)
    .range([left, Math.max(width - MARGIN_RIGHT, left)])
}

export function xTicks(
  scale: ReturnType<typeof xScale>,
  domain: [number, number],
  width: number,
  left: number,
): TimeTick[] {
  const [lo, hi] = domain

  // A single sample spans a millisecond, where every label reads the same.
  if (hi - lo < milliseconds({ seconds: 1 })) return []

  // Below two minutes the minute-resolution labels would all read the same.
  const mask = hi - lo < milliseconds({ minutes: 2 }) ? 'HH:mm:ss' : 'HH:mm'
  const right = width - MARGIN_RIGHT

  return scale.ticks(Math.max(Math.floor(width / 90), 2)).map((date) => {
    const x = scale(date)

    return {
      key: date.getTime(),
      label: formatDate(date, mask),
      x,
      // Pull the outermost labels inside the plot so they aren't clipped.
      anchor: x - left < LABEL_REACH ? 'start' : right - x < LABEL_REACH ? 'end' : 'middle',
    }
  })
}

/** Series as stacked bands; unstacked charts all sit on the zero baseline. */
export function bands(samples: Sample[], seriesCount: number, stacked?: boolean): Band[][] {
  return Array.from({ length: seriesCount }, (_, index) =>
    samples.map<Band>(({ t, values }) => {
      const y0 = stacked ? values.slice(0, index).reduce((sum, v) => sum + v, 0) : 0

      return { t, y0, y1: y0 + values[index] }
    }),
  )
}

export function areaPath(x: ReturnType<typeof xScale>, y: ReturnType<typeof yScale>) {
  return d3Area<Band>()
    .x((d) => x(d.t))
    .y0((d) => y(d.y0))
    .y1((d) => y(d.y1))
    .curve(curveMonotoneX)
}

export function linePath(x: ReturnType<typeof xScale>, y: ReturnType<typeof yScale>) {
  return d3Line<Band>()
    .x((d) => x(d.t))
    .y((d) => y(d.y1))
    .curve(curveMonotoneX)
}

/** Offset from the crosshair, pulled back in when it would overhang the plot. */
export function tooltipLeft(anchorX: number, tipWidth: number, plotWidth: number) {
  return Math.min(Math.max(anchorX + CURSOR_GAP, 0), Math.max(plotWidth - tipWidth, 0))
}

/** Index of the sample drawn closest to a cursor position. */
export function nearestIndex(samples: Sample[], scale: ReturnType<typeof xScale>, cursorX: number) {
  // A couple of dozen points, so a linear scan beats a bisector.
  let nearest = 0
  let nearestDistance = Infinity

  samples.forEach((sample, i) => {
    const distance = Math.abs(scale(sample.t) - cursorX)
    if (distance < nearestDistance) {
      nearestDistance = distance
      nearest = i
    }
  })

  return nearest
}
