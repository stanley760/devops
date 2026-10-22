<script setup lang="ts">
import type { HealthKind } from './types'

withDefaults(
  defineProps<{
    kind: HealthKind
    text: string
    led?: boolean
  }>(),
  { led: true },
)

const tagCls: Record<HealthKind, string | false> = {
  ok: 'tag-ok',
  warn: 'tag-warn',
  crit: 'tag-crit',
  off: false,
}

const ledCls: Record<HealthKind, string> = {
  ok: 'led led-ok',
  warn: 'led led-warn',
  crit: 'led led-crit',
  off: 'led',
}
</script>

<template>
  <span class="tag" :class="tagCls[kind]">
    <i v-if="led" :class="ledCls[kind]" />
    {{ text }}
  </span>
</template>
