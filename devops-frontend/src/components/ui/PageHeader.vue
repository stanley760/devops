<script setup lang="ts">
defineProps<{
  title: string
  desc?: string
}>()
</script>

<template>
  <div class="page-header">
    <div>
      <div class="page-title">{{ title }}</div>
      <div v-if="desc" class="page-desc">{{ desc }}</div>
      <slot />
    </div>
    <div class="page-actions">
      <slot name="actions" />
    </div>
  </div>
</template>
