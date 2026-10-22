<script setup lang="ts">
/**
 * 命令面板（⌘/Ctrl+K）：扁平化动态菜单的模糊检索。
 * 自持浮层（不依赖 Modal），动效走 motion 的 modal 语态；闭合时不渲染任何节点。
 */
import { computed, nextTick, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { navGroups, type NavLeaf } from '@/composables/auth'
import { overlayEnter, overlayLeave } from '@/composables/motion'
import Icon from '@/components/Icon.vue'

const props = defineProps<{ open: boolean }>()
const emit = defineEmits<{ close: [] }>()

const router = useRouter()

const keyword = ref('')
const active = ref(0)
const inputEl = ref<HTMLInputElement | null>(null)
/* 背板比面板晚卸一拍：关闭动效期间保持遮罩 */
const showBackdrop = ref(false)

const results = computed<NavLeaf[]>(() => {
  const all = navGroups.value.flatMap((g) => g.items)
  const k = keyword.value.trim().toLowerCase()
  if (!k) return all
  return all.filter(
    (i) => i.label.toLowerCase().includes(k) || i.path.toLowerCase().includes(k),
  )
})

watch(keyword, () => {
  active.value = 0
})

watch(
  () => props.open,
  (open) => {
    if (!open) return
    showBackdrop.value = true
    keyword.value = ''
    active.value = 0
    void nextTick(() => inputEl.value?.focus())
  },
)

function go(item: NavLeaf): void {
  if (item.path !== router.currentRoute.value.path) void router.push(item.path)
  emit('close')
}

function onKeydown(e: KeyboardEvent): void {
  const list = results.value
  if (e.key === 'Escape') {
    e.preventDefault()
    emit('close')
    return
  }
  if (e.key === 'ArrowDown') {
    e.preventDefault()
    if (list.length) active.value = Math.min(active.value + 1, list.length - 1)
    return
  }
  if (e.key === 'ArrowUp') {
    e.preventDefault()
    if (list.length) active.value = Math.max(active.value - 1, 0)
    return
  }
  if (e.key === 'Enter') {
    e.preventDefault()
    const item = list[active.value]
    if (item) go(item)
  }
}
</script>

<template>
  <Teleport to="body">
    <div v-if="showBackdrop" class="overlay-backdrop" @click="emit('close')" />

    <Transition
      :css="false"
      @enter="(el, done) => overlayEnter(el, done, 'modal')"
      @leave="(el, done) => overlayLeave(el, done, 'modal')"
      @after-leave="showBackdrop = false"
    >
      <div
        v-if="open"
        class="modal m-float"
        role="dialog"
        aria-modal="true"
        aria-label="搜索菜单"
        @keydown="onKeydown"
      >
        <div class="modal-body">
          <input
            ref="inputEl"
            v-model="keyword"
            class="input"
            type="text"
            placeholder="搜索菜单、路径…"
            spellcheck="false"
            autocomplete="off"
          />
          <div style="margin-top: 10px">
            <button
              v-for="(item, i) in results"
              :key="item.key"
              class="menu-item"
              :class="{ 'm-pill': i === active }"
              type="button"
              @mouseenter="active = i"
              @click="go(item)"
            >
              <Icon :name="item.icon || 'ep:menu'" :size="16" />
              <span>{{ item.label }}</span>
              <span class="mono" style="margin-left: auto; font-size: 11px; color: var(--ink-3)">
                {{ item.path }}
              </span>
            </button>
            <div v-if="results.length === 0" class="table-empty">无匹配菜单</div>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>
