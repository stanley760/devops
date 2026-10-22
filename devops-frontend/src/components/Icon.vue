<script setup lang="ts">
/**
 * SVG 图标解析器 —— 内联 path 表（24×24 描边）。未知名称回退为方框。
 */
import { computed } from 'vue'

const props = withDefaults(
  defineProps<{
    name: string
    size?: number
    stroke?: number
  }>(),
  { size: 18, stroke: 1.7 },
)

const paths: Record<string, string> = {
  'ep:setting': 'M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 1 1-4 0v-.09a1.65 1.65 0 0 0-1-1.51 1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 1 1 0-4h.09a1.65 1.65 0 0 0 1.51-1 1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33h.09a1.65 1.65 0 0 0 1-1.51V3a2 2 0 1 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82v.09a1.65 1.65 0 0 0 1.51 1H21a2 2 0 1 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1zM12 15a3 3 0 1 0 0-6 3 3 0 0 0 0 6z',
  'ep:user': 'M12 12a4 4 0 1 0 0-8 4 4 0 0 0 0 8zM4 20.5c.8-3.4 4-5.5 8-5.5s7.2 2.1 8 5.5',
  'ep:key': 'M16 8a4 4 0 1 1-4 5.9L5.5 20.5 3 23l1-3 2-2 2.1-2.1A4 4 0 0 1 16 8zM14 10h.01',
  'ep:menu': 'M4 6h16M4 12h16M4 18h16',
  'ep:office-building': 'M4 21V5a1 1 0 0 1 1-1h8a1 1 0 0 1 1 1v16M14 10h5a1 1 0 0 1 1 1v10M2 21h20M7 8h2M7 12h2M7 16h2M17 14h1M17 18h1',
  'ep:postcard': 'M3 5h18v14H3zM8 12a2 2 0 1 0 0-4 2 2 0 0 0 0 4zM5 16c.5-2 1.6-3 3-3s2.5 1 3 3M14 10h5M14 14h5',
  'ep:coin': 'M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18zM12 6v12M14.5 9.5c-.4-.9-1.4-1.5-2.5-1.5-1.4 0-2.5.9-2.5 2s1 1.8 2.5 2 2.5.9 2.5 2-1.1 2-2.5 2c-1.1 0-2.1-.6-2.5-1.5',
  'ep:document': 'M6 2h8l4 4v16H6zM14 2v4h4M9 12h6M9 16h6',
  'ep:collection': 'M12 2l9 5-9 5-9-5zM3 12l9 5 9-5M3 17l9 5 9-5',
  'ep:operation': 'M4 6h10M18 6h2M4 12h4M12 12h8M4 18h10M18 18h2M14 4v4M8 10v4M14 16v4',
  /** 滑杆：系统配置（与 ep:setting 齿轮区分，避免同组图标重复） */
  'ep:sliders': 'M4 7h9M17 7h3M4 17h3M11 17h9M15 4.5v5M8 14.5v5',
  'ep:odometer': 'M3.5 18.5a9 9 0 1 1 17 0M12 12l3.5-3.5M12 7v1M7 9.5l.7.7M17 9.5l-.7.7',
  'ep:plus': 'M12 5v14M5 12h14',
  'ep:edit': 'M12 20h9M16.5 3.5a2.1 2.1 0 0 1 3 3L7 19l-4 1 1-4 12.5-12.5z',
  'ep:delete': 'M3 6h18M8 6V4h8v2M6 6l1 14h10l1-14M10 10v6M14 10v6',
  'ep:search': 'M11 19a8 8 0 1 0 0-16 8 8 0 0 0 0 16zM21 21l-4.3-4.3',
  'ep:refresh': 'M21 12a9 9 0 1 1-2.64-6.36M21 3v6h-6',
  'ep:close': 'M6 6l12 12M18 6L6 18',
  'ep:arrow-left': 'M19 12H5M11 6l-6 6 6 6',
  'ep:arrow-right': 'M5 12h14M13 6l6 6-6 6',
  'ep:arrow-up': 'M12 19V5M6 11l6-6 6 6',
  'ep:arrow-down': 'M12 5v14M6 13l6 6 6-6',
  'ep:moon': 'M21 12.8A9 9 0 1 1 11.2 3a7 7 0 0 0 9.8 9.8z',
  'ep:sunny': 'M12 17a5 5 0 1 0 0-10 5 5 0 0 0 0 10zM12 1.5v3M12 19.5v3M4.6 4.6l2.1 2.1M17.3 17.3l2.1 2.1M1.5 12h3M19.5 12h3M4.6 19.4l2.1-2.1M17.3 6.7l2.1-2.1',
  'ep:monitor': 'M3 4h18v12H3zM8 20h8M12 16v4',
  'ep:bell': 'M18 8a6 6 0 1 0-12 0c0 7-3 9-3 9h18s-3-2-3-9M13.7 21a2 2 0 0 1-3.4 0',
  'ep:lock': 'M6 10h12v11H6zM8.5 10V7a3.5 3.5 0 1 1 7 0v3',
  'ep:grid': 'M3 3h7v7H3zM14 3h7v7h-7zM3 14h7v7H3zM14 14h7v7h-7z',
  'ep:folder': 'M3 6a2 2 0 0 1 2-2h4l2 3h8a2 2 0 0 1 2 2v9a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z',
  'ep:link': 'M10 13a5 5 0 0 0 7.5.5l3-3a5 5 0 0 0-7-7l-1.7 1.7M14 11a5 5 0 0 0-7.5-.5l-3 3a5 5 0 0 0 7 7l1.7-1.7',
  'ep:warning': 'M12 3L1.8 20h20.4zM12 9v5M12 17.5h.01',
  'ep:circle-check': 'M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18zM8 12.5l2.5 2.5L16 9.5',
  'ep:circle-close': 'M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18zM9 9l6 6M15 9l-6 6',
  'ep:download': 'M12 3v12M7 11l5 5 5-5M4 20h16',
  'ep:upload': 'M12 21V9M7 13l5-5 5 5M4 4h16',
  'ep:more': 'M5 12h.01M12 12h.01M19 12h.01',
  'ep:expand': 'M8 3H5a2 2 0 0 0-2 2v3M16 3h3a2 2 0 0 1 2 2v3M21 16v3a2 2 0 0 1-2 2h-3M3 16v3a2 2 0 0 0 2 2h3',
  'ep:fold': 'M8 3v3a2 2 0 0 1-2 2H3M16 3v3a2 2 0 0 0 2 2h3M21 16h-3a2 2 0 0 0-2 2v3M3 16h3a2 2 0 0 1 2 2v3',
  'ep:clock': 'M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18zM12 7v5l3 2',
  'ep:cpu': 'M6 6h12v12H6zM9 9h6v6H9zM9 2v2M15 2v2M9 20v2M15 20v2M2 9h2M2 15h2M20 9h2M20 15h2',
  'ep:connection': 'M12 6.5a2.5 2.5 0 1 0 0-5 2.5 2.5 0 0 0 0 5zM5 21.5a2.5 2.5 0 1 0 0-5 2.5 2.5 0 0 0 0 5zM19 21.5a2.5 2.5 0 1 0 0-5 2.5 2.5 0 0 0 0 5zM10.7 5.2L6.2 15.3M13.3 5.2l4.5 10.1M7.5 18.5h9',
  'ep:compass': 'M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18zM16 8l-2.5 5.5L8 16l2.5-5.5z',
}

const d = computed(() => paths[props.name] ?? 'M5 5h14v14H5z')
</script>

<template>
  <svg
    :width="size"
    :height="size"
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    :stroke-width="stroke"
    stroke-linecap="round"
    stroke-linejoin="round"
    aria-hidden="true"
  >
    <path :d="d" />
  </svg>
</template>
