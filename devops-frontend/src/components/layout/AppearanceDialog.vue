<script setup lang="ts">
/**
 * 外观设置：主题 / 密度 / 导航折叠，实时写入根元素偏好。
 */
import { appearance, setTheme, setDensity, toggleRail, type ThemeMode, type Density } from '@/composables/appearance'
import { Modal, Field, Segmented, Switch } from '@/components/ui'

defineProps<{ open: boolean }>()
const emit = defineEmits<{ close: [] }>()

const themeOptions: Array<{ value: ThemeMode; label: string }> = [
  { value: 'light', label: '浅色' },
  { value: 'dark', label: '深色' },
  { value: 'auto', label: '跟随系统' },
]
const densityOptions: Array<{ value: Density; label: string }> = [
  { value: 'comfy', label: '宽松' },
  { value: 'regular', label: '标准' },
  { value: 'compact', label: '紧凑' },
]

function onTheme(v: string | number): void {
  setTheme(v as ThemeMode)
}
function onDensity(v: string | number): void {
  setDensity(v as Density)
}
</script>

<template>
  <Modal :open="open" title="外观设置" hide-footer @close="emit('close')">
    <Field label="主题">
      <Segmented :model-value="appearance.theme" :options="themeOptions" @update:model-value="onTheme" />
    </Field>
    <Field label="密度" style="margin-top: 16px">
      <Segmented :model-value="appearance.density" :options="densityOptions" @update:model-value="onDensity" />
    </Field>
    <div
      class="field"
      style="margin-top: 16px; flex-direction: row; align-items: center; justify-content: space-between"
    >
      <span class="field-label">折叠导航栏</span>
      <Switch :model-value="appearance.railCollapsed" @update:model-value="toggleRail" />
    </div>
  </Modal>
</template>
