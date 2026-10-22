<script setup lang="ts">
import Icon from '../Icon.vue'

withDefaults(
  defineProps<{
    text?: string
    icon?: string
  }>(),
  {
    text: '暂无数据',
    icon: 'ep:folder',
  },
)
</script>

<template>
  <div class="empty">
    <div class="empty-icon"><Icon :name="icon" :size="24" /></div>
    <div>{{ text }}</div>
  </div>
</template>
