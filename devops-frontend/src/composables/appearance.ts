/**
 * 外观：浅色 / 深色 / 跟随系统 + 表格密度 + 导航折叠。
 * 偏好写在根元素 data-theme / data-density 上（无 class）。
 */
import { reactive, watchEffect } from 'vue'

export type ThemeMode = 'light' | 'dark' | 'auto'
export type Density = 'comfy' | 'regular' | 'compact'

export interface Appearance {
  theme: ThemeMode
  density: Density
  railCollapsed: boolean
}

const KEY = 'devops.appearance'

function load(): Appearance {
  try {
    const raw = localStorage.getItem(KEY)
    if (raw) return { theme: 'auto', density: 'regular', railCollapsed: false, ...JSON.parse(raw) }
  } catch {
    /* 坏数据忽略 */
  }
  return { theme: 'auto', density: 'regular', railCollapsed: false }
}

export const appearance = reactive<Appearance>(load())

function systemDark(): boolean {
  return window.matchMedia('(prefers-color-scheme: dark)').matches
}

function apply(): void {
  const root = document.documentElement
  const theme = appearance.theme === 'auto' ? (systemDark() ? 'dark' : 'light') : appearance.theme
  root.dataset.theme = theme
  root.dataset.density = appearance.density
  localStorage.setItem(KEY, JSON.stringify(appearance))
}

watchEffect(apply)

window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
  if (appearance.theme === 'auto') apply()
})

export function setTheme(mode: ThemeMode): void {
  appearance.theme = mode
}

export function setDensity(d: Density): void {
  appearance.density = d
}

export function toggleRail(): void {
  appearance.railCollapsed = !appearance.railCollapsed
}
