export type Theme = 'light' | 'dark' | 'auto'
export type Density = 'comfy' | 'regular' | 'compact'

export interface Appearance {
  theme: Theme
  density: Density
  railCollapsed: boolean
}

export const DEFAULT_APPEARANCE: Appearance = {
  theme: 'auto',
  density: 'regular',
  railCollapsed: false,
}

const KEY = 'devops.appearance'

export function loadAppearance(): Appearance {
  try {
    const raw = localStorage.getItem(KEY)
    if (!raw) return { ...DEFAULT_APPEARANCE }
    const parsed = JSON.parse(raw) as Partial<Appearance>
    return {
      theme: parsed.theme ?? DEFAULT_APPEARANCE.theme,
      density: parsed.density ?? DEFAULT_APPEARANCE.density,
      railCollapsed: parsed.railCollapsed ?? DEFAULT_APPEARANCE.railCollapsed,
    }
  } catch {
    return { ...DEFAULT_APPEARANCE }
  }
}

export function saveAppearance(a: Appearance): void {
  localStorage.setItem(KEY, JSON.stringify(a))
}

export function systemPrefersDark(): boolean {
  return typeof window !== 'undefined' && window.matchMedia('(prefers-color-scheme: dark)').matches
}

export function resolveDark(a: Appearance): boolean {
  return a.theme === 'dark' || (a.theme === 'auto' && systemPrefersDark())
}

export function applyAppearance(a: Appearance = loadAppearance()): void {
  const root = document.documentElement
  root.classList.toggle('dark', resolveDark(a))
  root.dataset.density = a.density
  root.classList.toggle('rail-collapsed', a.railCollapsed)
}

export function watchSystemTheme(cb: (a: Appearance) => void): () => void {
  const mql = window.matchMedia('(prefers-color-scheme: dark)')
  const onChange = () => cb(loadAppearance())
  mql.addEventListener('change', onChange)
  return () => mql.removeEventListener('change', onChange)
}

import { useCallback, useEffect, useState } from 'react'

export function useAppearance(): { appearance: Appearance; update: (patch: Partial<Appearance>) => void } {
  const [appearance, setAppearance] = useState<Appearance>(() => loadAppearance())

  useEffect(() => {
    return watchSystemTheme((a) => {
      setAppearance(a)
      applyAppearance(a)
    })
  }, [])

  const update = useCallback((patch: Partial<Appearance>) => {
    setAppearance((prev) => {
      const next = { ...prev, ...patch }
      saveAppearance(next)
      applyAppearance(next)
      return next
    })
  }, [])

  return { appearance, update }
}
