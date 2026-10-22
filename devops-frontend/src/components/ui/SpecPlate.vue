<script setup lang="ts">
import type { SpecRow } from './types'

withDefaults(
  defineProps<{
    title: string
    rows: SpecRow[]
    light?: boolean
  }>(),
  { light: false },
)
</script>

<template>
  <div class="spec-plate m-plate" :class="light && 'is-light'">
    <div class="sp-title">{{ title }}</div>
    <div class="sp-rows">
      <div v-for="row in rows" :key="row.key" class="sp-row">
        <span class="sp-key">{{ row.key }}</span>
        <span class="sp-val">{{ row.val }}</span>
      </div>
    </div>
  </div>
</template>
