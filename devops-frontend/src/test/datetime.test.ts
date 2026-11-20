import { describe, it, expect } from 'vitest'
import { formatDateTime, formatDate } from '@/lib/datetime'

describe('formatDateTime', () => {
  it('formats unix millis', () => {
    expect(formatDateTime(1730000000000)).toMatch(/^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$/)
  })
  it('formats string dates', () => {
    expect(formatDateTime('2026-01-02 03:04:05')).toBe('2026-01-02 03:04:05')
  })
  it('returns dash for null/empty', () => {
    expect(formatDateTime(null)).toBe('-')
    expect(formatDateTime('')).toBe('-')
  })
})

describe('formatDate', () => {
  it('returns date part only', () => {
    expect(formatDate('2026-01-02 03:04:05')).toBe('2026-01-02')
  })
})
