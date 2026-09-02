import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import StatsBarChart from '../StatsBarChart.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

const makePoints = (count: number) => Array.from({ length: count }, (_, index) => ({
  date: new Date(Date.UTC(2026, 7, 25, index)).toISOString(),
  value: index + 1,
}))

const styleNumber = (style: string, property: string) => {
  const match = style.match(new RegExp(`${property}: ([\\d.]+)%`))
  if (!match) throw new Error(`Missing ${property} in ${style}`)
  return Number.parseFloat(match[1])
}

describe('StatsBarChart band geometry', () => {
  for (const count of [1, 2, 8, 12, 24]) {
    it(`keeps ${count} bars inside their non-overlapping slots`, () => {
      const wrapper = mount(StatsBarChart, {
        props: {
          title: 'Requests',
          unit: 'Requests',
          points: makePoints(count),
          granularity: 'hour',
          testId: `chart-${count}`,
        },
      })

      const intervals = wrapper.findAll('[data-testid="stats-trend-bar"]').map((bar) => {
        const style = bar.attributes('style') ?? ''
        const center = styleNumber(style, 'left')
        const width = styleNumber(style, 'width')
        return { left: center - width / 2, right: center + width / 2, width }
      })

      expect(intervals).toHaveLength(count)
      expect(intervals[0].left).toBeGreaterThanOrEqual(12)
      expect(intervals.at(-1)!.right).toBeLessThanOrEqual(92)
      expect(intervals[0].width).toBeLessThanOrEqual(14)
      for (let index = 1; index < intervals.length; index += 1) {
        expect(intervals[index].left).toBeGreaterThan(intervals[index - 1].right)
      }
    })
  }

  it('keeps hover, focus, and ordered keyboard navigation on individual bars', async () => {
    const wrapper = mount(StatsBarChart, {
      attachTo: document.body,
      props: {
        title: 'Requests',
        unit: 'Requests',
        points: makePoints(3),
        granularity: 'hour',
        testId: 'interactive-chart',
      },
    })
    const bars = wrapper.findAll<HTMLButtonElement>('[data-testid="stats-trend-bar"]')

    await bars[1].trigger('mouseenter')
    expect(bars[1].classes()).toContain('is-active')
    expect(bars[0].classes()).not.toContain('is-active')
    expect(wrapper.get('[data-testid="stats-trend-tooltip"]').text()).toContain('2 Requests')

    await bars[1].trigger('mouseleave')
    await bars[0].trigger('focus')
    await bars[0].trigger('keydown', { key: 'ArrowRight' })
    expect(document.activeElement).toBe(bars[1].element)

    wrapper.unmount()
  })
})
