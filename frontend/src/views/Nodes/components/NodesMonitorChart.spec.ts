// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { mount } from '@vue/test-utils'
import { expect, test, vi } from 'vitest'
import { ref } from 'vue'
import type { ComponentProps } from 'vue-component-type-helpers'

import { Runtime } from '@/api/common/omni.pb'
import { EventType } from '@/api/omni/resources/resources.pb'
import type { WatchOptions } from '@/methods/useResourceWatch'

const WIDTH = 400
const HEIGHT = 180

// jsdom reports a zero-sized element, which collapses the scales.
vi.mock('@vueuse/core', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@vueuse/core')>()),
  useElementSize: () => ({ width: ref(WIDTH), height: ref(HEIGHT) }),
}))

// ...and a zero-sized rect, which would put the cursor permanently outside.
Element.prototype.getClientRects = () =>
  [{ left: 0, top: 0, width: WIDTH, height: HEIGHT }] as unknown as DOMRectList

/** The chart tracks the window, so move the cursor rather than the element. */
const moveCursorTo = async (wrapper: ReturnType<typeof mount>, x: number) => {
  window.dispatchEvent(new MouseEvent('mousemove', { clientX: x, clientY: HEIGHT / 2 }))
  await wrapper.vm.$nextTick()
}

const moveCursorAway = async (wrapper: ReturnType<typeof mount>) => {
  window.dispatchEvent(new MouseEvent('mousemove', { clientX: -1, clientY: -1 }))
  await wrapper.vm.$nextTick()
}

const loading = ref(false)
let emit: (version: number) => void

vi.mock('@/methods/useResourceWatch', () => ({
  useResourceWatch: (
    _options: unknown,
    { onMessage }: { onMessage: (...args: never[]) => void },
  ) => {
    emit = (version) =>
      (onMessage as (...args: unknown[]) => void)(
        { event: { event_type: EventType.UPDATED } },
        {
          res: {
            spec: { tick: version },
            metadata: {
              version: String(version),
              updated: new Date(Date.UTC(2026, 0, 1, 12, 0, version)).toISOString(),
            },
          },
          old: { spec: { tick: version - 1 } },
        },
      )

    return { err: ref(null), errCode: ref(null), loading }
  },
}))

const { default: NodesMonitorChart } = await import('./NodesMonitorChart.vue')
type ChartSeries = import('./NodesMonitorChart.vue').ChartSeries

const watchOpts: WatchOptions = {
  runtime: Runtime.Talos,
  resource: { type: 'type', namespace: 'namespace', id: 'id' },
  context: { cluster: 'cluster', node: 'node' },
}

type ChartProps = ComponentProps<typeof NodesMonitorChart>

const render = async (
  props: Partial<ChartProps> & Pick<ChartProps, 'point' | 'series'>,
  points = 3,
) => {
  const wrapper = mount(NodesMonitorChart, {
    props: { title: 'Chart', watchOpts, ...props },
  })

  for (let version = 1; version <= points; version++) emit(version)
  await wrapper.vm.$nextTick()

  return wrapper
}

const named = (...keys: string[]): ChartSeries[] =>
  keys.map((key, i) => ({ key, color: `var(--color-${i})` }))

test('shows the hovered point in the tooltip, topmost series first when stacked', async () => {
  const wrapper = await render(
    {
      stacked: true,
      series: [
        { key: 'system', label: 'System', color: 'red' },
        { key: 'user', label: 'User', color: 'blue' },
      ],
      point: () => ({ system: 8, user: 16 }),
      min: 0,
      max: 100,
      format: (value: number) => `${value.toFixed(1)} %`,
    },
    8,
  )

  await moveCursorTo(wrapper, WIDTH)

  const tooltip = wrapper.find('.bg-naturals-n3')
  expect(tooltip.text()).toContain('User16.0 %')
  expect(tooltip.text()).toContain('System8.0 %')
  expect(tooltip.text().indexOf('User')).toBeLessThan(tooltip.text().indexOf('System'))

  await moveCursorAway(wrapper)
  expect(wrapper.find('.bg-naturals-n3').exists()).toBe(false)
})

test('keeps the readout on the sample nearest the cursor as points stream in', async () => {
  // A full window, so the tail slides without the x domain also growing.
  const wrapper = await render(
    { series: named('cpu'), point: () => ({ cpu: 40 }), min: 0, max: 100 },
    25,
  )

  await moveCursorTo(wrapper, 250)
  const crosshairX = () => wrapper.find('g line.stroke-naturals-n8').attributes('x1')
  const readout = () => wrapper.find('.bg-naturals-n3').text()

  const pinned = crosshairX()
  const first = readout()

  // More points arrive while the cursor stays put.
  emit(26)
  emit(27)
  await wrapper.vm.$nextTick()

  // Stays on the sample under the cursor rather than a frozen index, so the
  // crosshair holds its place while the sample it reports moves on.
  expect(crosshairX()).toBe(pinned)
  expect(readout()).not.toBe(first)
})

test('hands the point function the current and previous specs by name', async () => {
  const seen: { spec: unknown; previous: unknown }[] = []

  await render(
    {
      series: named('value'),
      point: ({ spec, previous }) => {
        seen.push({ spec, previous })
        return { value: (spec as { tick: number }).tick }
      },
    },
    3,
  )

  expect(seen).toEqual([
    { spec: { tick: 1 }, previous: { tick: 0 } },
    { spec: { tick: 2 }, previous: { tick: 1 } },
    { spec: { tick: 3 }, previous: { tick: 2 } },
  ])
})

test('clears collected points when the watch restarts', async () => {
  const wrapper = await render({ series: named('a'), point: () => ({ a: 1 }) })
  const area = () => wrapper.find('path[fill^="url"]').attributes('d')

  expect(area()).not.toBe('')

  loading.value = true
  await wrapper.vm.$nextTick()
  loading.value = false
  await wrapper.vm.$nextTick()

  // The declared series keep their paths; only the collected points go.
  expect(area()).toBe('')
})
