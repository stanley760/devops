<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { slideThumb } from '@/composables/motion'

interface SegOption {
  value: string | number
  label: string
}

const props = defineProps<{
  options: SegOption[]
  modelValue: string | number
}>()

const emit = defineEmits<{
  'update:modelValue': [value: string | number]
}>()

const root = ref<HTMLElement | null>(null)
const thumb = ref<HTMLElement | null>(null)
let ro: ResizeObserver | null = null

function playThumb(instant = false): void {
  if (!thumb.value || !root.value) return
  const idx = Math.max(
    0,
    props.options.findIndex((o) => o.value === props.modelValue),
  )
  slideThumb(thumb.value, `[data-idx="${idx}"]`, 'x', instant)
}

onMounted(() => {
  playThumb(true)
  ro = new ResizeObserver(() => playThumb(true))
  if (root.value) ro.observe(root.value)
})

watch(
  () => props.modelValue,
  () => playThumb(),
)

onBeforeUnmount(() => {
  ro?.disconnect()
  ro = null
})
</script>

<template>
  <div ref="root" class="segmented">
    <span ref="thumb" class="seg-thumb" />
    <button
      v-for="(opt, i) in options"
      :key="opt.value"
      type="button"
      :data-idx="i"
      :class="{ 'is-active': opt.value === modelValue }"
      @click="emit('update:modelValue', opt.value)"
    >
      {{ opt.label }}
    </button>
  </div>
</template>
