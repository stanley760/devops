<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { countUp } from '@/composables/motion'

const props = withDefaults(
  defineProps<{
    value: number
    decimals?: number
    duration?: number
  }>(),
  {
    decimals: 0,
    duration: 0.85,
  },
)

const el = ref<HTMLElement | null>(null)

function play(): void {
  if (!el.value) return
  countUp(el.value, props.value, { decimals: props.decimals, duration: props.duration })
}

onMounted(play)
watch(() => props.value, play)
</script>

<template>
  <span ref="el" class="odo" />
</template>
