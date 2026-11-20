import { renderHook, act } from '@testing-library/react'
import { describe, it, expect } from 'vitest'
import { useAppearance, loadAppearance } from '@/lib/appearance'

describe('useAppearance', () => {
  it('persists patch to localStorage and applies to html', () => {
    const { result } = renderHook(() => useAppearance())
    act(() => result.current.update({ theme: 'dark', railCollapsed: true }))
    expect(result.current.appearance.theme).toBe('dark')
    expect(document.documentElement.classList.contains('dark')).toBe(true)
    expect(document.documentElement.classList.contains('rail-collapsed')).toBe(true)
    expect(loadAppearance().theme).toBe('dark')
  })
})
