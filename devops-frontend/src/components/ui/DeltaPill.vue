<script setup lang="ts">
const props = withDefaults(
  defineProps<{
    value: number
    suffix?: string
    fresh?: boolean
  }>(),
  {
    suffix: '%',
    fresh: false,
  },
)

const text = (): string => {
  if (props.fresh) return '新增'
  const arrow = props.value > 0 ? '↑' : props.value < 0 ? '↓' : ''
  return `${arrow}${Math.abs(props.value).toFixed(1)}${props.suffix}`
}
</script>

<template>
  <span
    class="delta-pill"
    :class="{
      'is-fresh': fresh,
      'is-up': value > 0,
      'is-down': value < 0,
    }"
  >
    {{ text() }}
  </span>
</template>
