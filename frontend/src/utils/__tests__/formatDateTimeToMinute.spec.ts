import { describe, expect, it } from 'vitest'

import { formatDateTimeToMinute, formatDayAwareDateTime } from '../format'

describe('formatDateTimeToMinute', () => {
  it('formats local date and time without seconds', () => {
    const value = new Date(2026, 6, 19, 20, 30, 45)

    expect(formatDateTimeToMinute(value, 'en-GB')).toBe('19/07/2026, 20:30')
  })

  it('returns an empty string for an invalid date', () => {
    expect(formatDateTimeToMinute(new Date('invalid'), 'en-GB')).toBe('')
  })
})

describe('formatDayAwareDateTime', () => {
  it('uses time today, a relative tomorrow label, and a compact later date', () => {
    const now = new Date(2026, 7, 30, 10, 0)

    expect(formatDayAwareDateTime(new Date(2026, 7, 30, 15, 0), 'en-US', now)).toBe('3:00 PM')
    expect(formatDayAwareDateTime(new Date(2026, 7, 31, 15, 0), 'en-US', now)).toBe('tomorrow 3:00 PM')
    expect(formatDayAwareDateTime(new Date(2026, 8, 2, 15, 0), 'en-US', now)).toBe('Sep 2, 3:00 PM')
    expect(formatDayAwareDateTime(new Date('invalid'), 'en-US', now)).toBe('')
  })
})
