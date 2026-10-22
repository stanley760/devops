<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { countUp } from '@/composables/motion'
import DeltaPill from './DeltaPill.vue'
import type { MetricDelta } from './types'

const props = withDefaults(
  defineProps<{
    label: string
    value: number | string
    unit?: string
    sub?: string
    hero?: boolean
    plate?: boolean
    delta?: MetricDelta
  }>(),
  {
    unit: '',
    sub: '',
    hero: false,
    plate: false,
    delta: undefined,
  },
)

const numEl = ref<HTMLElement | null>(null)

function play(): void {
  if (!numEl.value || typeof props.value !== 'number') return
  const decimals = Number.isInteger(props.value) ? 0 : 1
  countUp(numEl.value, props.value, { decimals })
}

onMounted(play)
watch(() => props.value, play)
</script>

<template>
  <div class="readout" :class="{ 'is-hero': hero, 'is-plate': plate }">
    <div class="ro-eyebrow">
      <span>{{ label }}</span>
      <DeltaPill
        v-if="delta"
        :value="delta.value"
        :suffix="delta.suffix"
        :fresh="delta.kind === 'fresh'"
        style="margin-left: auto"
      />
    </div>
    <div class="ro-value">
      <span v-if="typeof value === 'number'" ref="numEl" />
      <span v-else>{{ value }}</span>
      <span v-if="unit" class="ro-unit">{{ unit }}</span>
    </div>
    <div v-if="sub" class="ro-sub">{{ sub }}</div>
  </div>
</template>
