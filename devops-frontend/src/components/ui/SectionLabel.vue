<script setup lang="ts">
defineProps<{ title: string }>()
</script>

<template>
  <div class="section-label">
    <div class="sl-title">{{ title }}</div>
    <div class="sl-line" />
  </div>
</template>
