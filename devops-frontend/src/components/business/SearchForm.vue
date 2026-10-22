<script setup lang="ts">
/** 列表页搜索栏：字段由调用方以默认插槽注入，这里只负责玻璃底座与查询/重置动作。 */
import { Button } from '@/components/ui'
import Icon from '@/components/Icon.vue'

const emit = defineEmits<{ search: []; reset: [] }>()
</script>

<template>
  <div class="search-form m-glass">
    <slot />
    <div class="search-actions">
      <Button variant="secondary" @click="emit('reset')">
        <template #prefix><Icon name="ep:refresh" :size="16" /></template>
        重置
      </Button>
      <Button variant="primary" @click="emit('search')">
        <template #prefix><Icon name="ep:search" :size="16" /></template>
        查询
      </Button>
    </div>
  </div>
</template>
