<script setup lang="ts">
withDefaults(
  defineProps<{
    name: string
    src?: string
    size?: number
  }>(),
  {
    src: '',
    size: 32,
  },
)
</script>

<template>
  <img
    v-if="src"
    class="avatar"
    :src="src"
    :alt="name"
    :style="{ width: `${size}px`, height: `${size}px` }"
  />
  <span
    v-else
    class="avatar"
    :title="name"
    :style="{ width: `${size}px`, height: `${size}px`, fontSize: `${size * 0.42}px` }"
  >
    {{ name.charAt(0) }}
  </span>
</template>
