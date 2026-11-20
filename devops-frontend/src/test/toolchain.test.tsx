import { render, screen } from '@testing-library/react'
import { describe, it, expect, vi } from 'vitest'
import { applyAppearance, resolveDark, DEFAULT_APPEARANCE } from '@/lib/appearance'

describe('test toolchain + appearance bootstrap', () => {
  it('renders a React 19 component', () => {
    render(<div>hello</div>)
    expect(screen.getByText('hello')).toBeInTheDocument()
  })

  it('applyAppearance toggles html.dark for dark theme', () => {
    applyAppearance({ ...DEFAULT_APPEARANCE, theme: 'dark' })
    expect(document.documentElement.classList.contains('dark')).toBe(true)
  })

  it('resolveDark follows system in auto mode', () => {
    const spy = vi.spyOn(window, 'matchMedia').mockReturnValue({
      matches: true,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    } as unknown as MediaQueryList)
    expect(resolveDark({ theme: 'auto', density: 'regular', railCollapsed: false })).toBe(true)
    spy.mockRestore()
  })
})
