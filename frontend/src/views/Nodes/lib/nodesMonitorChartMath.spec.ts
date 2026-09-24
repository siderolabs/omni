// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { describe, expect, test } from 'vitest'

import * as chart from './nodesMonitorChartMath'

const at = (t: number, ...values: number[]) => ({ t, values })

describe('bands', () => {
  test('stacks each series onto the sum of the ones below it', () => {
    expect(chart.bands([at(0, 20, 30)], 2, true)).toEqual([
      [{ t: 0, y0: 0, y1: 20 }],
      [{ t: 0, y0: 20, y1: 50 }],
    ])
  })

  test('puts every series on the zero baseline when not stacked', () => {
    expect(chart.bands([at(0, 20, 30)], 2)).toEqual([
      [{ t: 0, y0: 0, y1: 20 }],
      [{ t: 0, y0: 0, y1: 30 }],
    ])
  })
})

describe('appendSample', () => {
  test('keeps only the most recent updates', () => {
    const grown = [1, 2, 3, 4, 5, 6].reduce(
      (samples, t) => chart.appendSample(samples, at(t), 3),
      [] as chart.Sample[],
    )

    expect(grown.map((s) => s.t)).toEqual([4, 5, 6])
  })

  test('leaves a short stream untouched', () => {
    expect(chart.appendSample([at(1)], at(2), 25).map((s) => s.t)).toEqual([1, 2])
  })
})

describe('yDomain', () => {
  test('uses explicit bounds as given', () => {
    expect(chart.yDomain([at(0, 40)], false, 0, 100)).toEqual([0, 100])
  })

  test('grows to the tallest stack when no maximum is set', () => {
    expect(chart.yDomain([at(0, 3, 4, 5), at(1, 1, 1, 1)], true, undefined, undefined)).toEqual([
      0, 12,
    ])
  })

  test('grows to the tallest single value when not stacked', () => {
    expect(chart.yDomain([at(0, 3, 13, 5)], false, undefined, undefined)).toEqual([0, 13])
  })

  test('never drops below the minimum', () => {
    expect(chart.yDomain([at(0, 1, 2)], false, 50, undefined)).toEqual([50, 51])
  })

  test('widens a zero-height domain, which would otherwise flatten the scale', () => {
    expect(chart.yDomain([], false, 0, 0)).toEqual([0, 1])
    expect(chart.yDomain([], false, 50, 50)).toEqual([50, 51])
  })
})

describe('yScale', () => {
  const HEIGHT = 180

  test('maps the domain across the plot, top to bottom', () => {
    const scale = chart.yScale([0, 100], HEIGHT, false)

    expect(scale(0)).toBe(HEIGHT - chart.MARGIN_BOTTOM)
    expect(scale(100)).toBe(chart.MARGIN_TOP)
  })

  test('rounds the top up to the next tick when the maximum is automatic', () => {
    expect(chart.yScale([0, 13], HEIGHT, true).domain()).toEqual([0, 15])
    expect(chart.yScale([0, 13], HEIGHT, false).domain()).toEqual([0, 13])
  })
})

describe('yTickValues', () => {
  test('splits explicit bounds evenly, so a rescaling format lands on round numbers', () => {
    const domain: [number, number] = [0, 16 * 1024 * 1024]
    const values = chart.yTickValues(chart.yScale(domain, 180, false), domain, true)

    expect(values.map((v) => v / 1024 / 1024)).toEqual([0, 4, 8, 12, 16])
  })

  test('defers to d3 when a bound is automatic', () => {
    const domain: [number, number] = [0, 13]

    expect(chart.yTickValues(chart.yScale(domain, 180, true), domain, false)).toEqual([
      0, 5, 10, 15,
    ])
  })
})

describe('resolveBound', () => {
  test('returns a fixed bound without needing a sample', () => {
    expect(chart.resolveBound(100, undefined)).toBe(100)
  })

  test('reads a function bound from the sample', () => {
    expect(chart.resolveBound((s: { cap: number }) => s.cap, { cap: 200 })).toBe(200)
  })

  test('waits for a sample before calling a function bound', () => {
    expect(chart.resolveBound((s: { cap: number }) => s.cap, undefined)).toBeUndefined()
  })

  test('keeps a bound of zero rather than treating it as absent', () => {
    expect(chart.resolveBound(0, undefined)).toBe(0)
    expect(chart.resolveBound(() => 0, {})).toBe(0)
  })
})

describe('formatValue', () => {
  test('drops a trailing .0 but keeps a real decimal', () => {
    expect(chart.formatValue(13)).toBe('13')
    expect(chart.formatValue(13.25)).toBe('13.3')
    expect(chart.formatValue(0)).toBe('0')
  })

  test('only strips the decimal at the end', () => {
    expect(chart.formatValue(10.04)).toBe('10')
  })

  test('defers to a supplied format', () => {
    expect(chart.formatValue(40, (v) => `${v} %`)).toBe('40 %')
  })
})

describe('nearestIndex', () => {
  const samples = [at(0), at(100), at(200), at(300)]
  const scale = chart.xScale(chart.xDomain(samples), 400, 0)

  test('finds the sample drawn closest to the cursor', () => {
    expect(chart.nearestIndex(samples, scale, scale(0))).toBe(0)
    expect(chart.nearestIndex(samples, scale, scale(300))).toBe(3)
    expect(chart.nearestIndex(samples, scale, scale(200) - 1)).toBe(2)
  })

  test('clamps to the ends rather than running off', () => {
    expect(chart.nearestIndex(samples, scale, -999)).toBe(0)
    expect(chart.nearestIndex(samples, scale, 999)).toBe(3)
  })
})

describe('tooltipLeft', () => {
  test('sits just clear of the crosshair', () => {
    expect(chart.tooltipLeft(100, 140, 400)).toBe(112)
  })

  test('pulls back so the tooltip does not overhang the right edge', () => {
    expect(chart.tooltipLeft(380, 140, 400)).toBe(260)
  })

  test('tracks the tooltip width rather than assuming one', () => {
    expect(chart.tooltipLeft(380, 80, 400)).toBe(320)
  })

  test('never goes negative, even when the tooltip is wider than the plot', () => {
    expect(chart.tooltipLeft(10, 500, 400)).toBe(0)
    expect(chart.tooltipLeft(-999, 140, 400)).toBe(0)
  })
})

describe('xDomain', () => {
  test('spans the first and last sample', () => {
    expect(chart.xDomain([at(10), at(20), at(30)])).toEqual([10, 30])
  })

  test('widens a single sample, which would otherwise be zero-width', () => {
    expect(chart.xDomain([at(10)])).toEqual([10, 11])
  })

  test('falls back to a unit domain with no samples', () => {
    expect(chart.xDomain([])).toEqual([0, 1])
  })
})

describe('xTicks', () => {
  const WIDTH = 400
  const domain: [number, number] = [0, 30_000]
  const ticks = chart.xTicks(chart.xScale(domain, WIDTH, 40), domain, WIDTH, 40)

  test('anchors the outermost labels inwards so they are not clipped', () => {
    expect(ticks[0].anchor).toBe('start')
    expect(ticks[ticks.length - 1].anchor).toBe('end')
    expect(ticks.slice(1, -1).every((tick) => tick.anchor === 'middle')).toBe(true)
  })

  test('uses second resolution for a short window', () => {
    expect(ticks[0].label).toMatch(/^\d{2}:\d{2}:\d{2}$/)
  })

  test('draws nothing until the window spans a second, where labels would repeat', () => {
    const single = chart.xDomain([at(1_700_000_000_000)])

    expect(chart.xTicks(chart.xScale(single, WIDTH, 40), single, WIDTH, 40)).toEqual([])
  })

  test('anchors against the plot edge rather than the svg edge', () => {
    // The rightmost tick sits on the plot edge, MARGIN_RIGHT short of the svg.
    const last = ticks[ticks.length - 1]

    expect(WIDTH - chart.MARGIN_RIGHT - last.x).toBeLessThan(1)
    expect(last.anchor).toBe('end')
  })

  test('uses minute resolution once the window is long', () => {
    const wide: [number, number] = [0, 60 * 60 * 1000]

    expect(chart.xTicks(chart.xScale(wide, WIDTH, 40), wide, WIDTH, 40)[0].label).toMatch(
      /^\d{2}:\d{2}$/,
    )
  })
})

describe('marginLeft', () => {
  test('reserves a gutter for the longest label', () => {
    expect(chart.marginLeft(['0', '1024 MiB'])).toBeGreaterThan(chart.marginLeft(['0', '100']))
  })

  test('holds a minimum with no labels', () => {
    expect(chart.marginLeft([])).toBeGreaterThan(0)
  })
})
