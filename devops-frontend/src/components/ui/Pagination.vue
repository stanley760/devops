<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(
  defineProps<{
    page: number
    pageSize: number
    total: number
    pageSizes?: number[]
  }>(),
  {
    pageSizes: () => [10, 20, 50],
  },
)

const emit = defineEmits<{
  'update:page': [page: number]
  'update:pageSize': [size: number]
}>()

const pages = computed(() => Math.max(1, Math.ceil(props.total / props.pageSize)))

const items = computed<Array<number | '…'>>(() => {
  const n = pages.value
  const p = Math.min(Math.max(1, props.page), n)
  const start = Math.max(1, Math.min(p - 2, n - 4))
  const end = Math.min(n, start + 4)
  const out: Array<number | '…'> = []
  if (start > 1) out.push('…')
  for (let i = start; i <= end; i++) out.push(i)
  if (end < n) out.push('…')
  return out
})

function goto(p: number): void {
  const next = Math.min(Math.max(1, p), pages.value)
  if (next !== props.page) emit('update:page', next)
}

function onSize(e: Event): void {
  emit('update:pageSize', Number((e.target as HTMLSelectElement).value))
}
</script>

<template>
  <div class="pagination">
    <div class="pg-info">共 {{ total }} 条 / {{ pages }} 页</div>
    <button type="button" class="pg-btn" :disabled="page <= 1" @click="goto(page - 1)">‹</button>
    <template v-for="(it, i) in items" :key="i">
      <span v-if="it === '…'" class="pg-btn" style="pointer-events: none">…</span>
      <button
        v-else
        type="button"
        class="pg-btn"
        :class="{ 'is-active': it === page }"
        @click="goto(it)"
      >
        {{ it }}
      </button>
    </template>
    <button type="button" class="pg-btn" :disabled="page >= pages" @click="goto(page + 1)">›</button>
    <select class="select" :value="pageSize" style="width: 84px" @change="onSize">
      <option v-for="s in pageSizes" :key="s" :value="s">{{ s }} 条/页</option>
    </select>
  </div>
</template>
