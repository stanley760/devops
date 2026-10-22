<script setup lang="ts">
/**
 * 顶栏：折叠 / 面包屑 / 命令面板入口 / 外观 / 用户菜单。
 * 用户浮层走 pop 动效，支持方向键漫游与点击外部关闭。
 */
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { currentUser, currentTenant } from '@/composables/auth'
import { overlayEnter, overlayLeave } from '@/composables/motion'
import { Avatar, IconButton } from '@/components/ui'
import Icon from '@/components/Icon.vue'

const emit = defineEmits<{
  'toggle-rail': []
  command: []
  appearance: []
  password: []
  logout: []
}>()

const route = useRoute()

const isMac = typeof navigator !== 'undefined' && navigator.platform.includes('Mac')

const crumbGroup = computed(() => String(route.meta.group ?? ''))
const crumbTitle = computed(() => String(route.meta.title ?? ''))

const displayName = computed(() => currentUser.value?.nickname || currentUser.value?.username || '未登录')
const tenantCode = computed(() => currentTenant.value?.code || 'default')

/* ---------------- 用户浮层 ---------------- */

const chipEl = ref<HTMLElement | null>(null)
const popEl = ref<HTMLElement | null>(null)
const menuOpen = ref(false)
const popStyle = ref<Record<string, string>>({})

function placePop(): void {
  const rect = chipEl.value?.getBoundingClientRect()
  if (!rect) return
  popStyle.value = {
    top: `${rect.bottom + 8}px`,
    right: `${Math.max(8, window.innerWidth - rect.right)}px`,
  }
}

function toggleMenu(): void {
  if (menuOpen.value) {
    menuOpen.value = false
    return
  }
  placePop()
  menuOpen.value = true
}

function pick(action: 'password' | 'appearance' | 'logout'): void {
  menuOpen.value = false
  // 具名 emit 重载不吃联合类型，按字面量分发
  if (action === 'password') emit('password')
  else if (action === 'appearance') emit('appearance')
  else emit('logout')
}

function onDocPointerDown(e: PointerEvent): void {
  const t = e.target as Node | null
  if (t && (popEl.value?.contains(t) || chipEl.value?.contains(t))) return
  menuOpen.value = false
}

/** 漫游键盘：上下循环菜单项，Escape 关闭并回焦头像。 */
function onPopKeydown(e: KeyboardEvent): void {
  const items = Array.from(popEl.value?.querySelectorAll<HTMLElement>('.menu-item') ?? [])
  if (e.key === 'Escape') {
    e.preventDefault()
    menuOpen.value = false
    chipEl.value?.focus()
    return
  }
  if (!items.length) return
  const idx = items.findIndex((el) => el === document.activeElement)
  if (e.key === 'ArrowDown') {
    e.preventDefault()
    items[(idx + 1) % items.length]?.focus()
  } else if (e.key === 'ArrowUp') {
    e.preventDefault()
    items[(idx - 1 + items.length) % items.length]?.focus()
  }
}

watch(menuOpen, async (open) => {
  if (open) {
    document.addEventListener('pointerdown', onDocPointerDown, true)
    await nextTick()
    popEl.value?.querySelector<HTMLElement>('.menu-item')?.focus()
  } else {
    document.removeEventListener('pointerdown', onDocPointerDown, true)
  }
})

onBeforeUnmount(() => {
  document.removeEventListener('pointerdown', onDocPointerDown, true)
})
</script>

<template>
  <header class="topbar">
    <div class="topbar-glass" aria-hidden="true" />

    <IconButton icon="ep:fold" label="折叠导航" @click="emit('toggle-rail')" />

    <div class="crumbs">
      <template v-if="crumbGroup">
        <span>{{ crumbGroup }}</span>
        <span aria-hidden="true">·</span>
      </template>
      <span class="crumb-now">{{ crumbTitle }}</span>
    </div>

    <div class="topbar-right">
      <button class="cmd-trigger m-pill" type="button" @click="emit('command')">
        <Icon name="ep:search" :size="16" />
        <span>搜索菜单</span>
        <span class="cmd-kbd" aria-hidden="true">
          <kbd>{{ isMac ? '⌘' : 'Ctrl' }}</kbd><kbd>K</kbd>
        </span>
      </button>

      <IconButton icon="ep:moon" label="外观设置" @click="emit('appearance')" />

      <button
        ref="chipEl"
        class="user-chip m-pill"
        type="button"
        :aria-expanded="menuOpen"
        aria-haspopup="menu"
        @click="toggleMenu"
      >
        <Avatar :name="displayName" :size="28" />
        <span class="chip-meta">
          <span>{{ displayName }}</span>
          <span class="chip-role">租户 {{ tenantCode }}</span>
        </span>
      </button>
    </div>

    <Teleport to="body">
      <Transition
        :css="false"
        @enter="(el, done) => overlayEnter(el, done, 'pop')"
        @leave="(el, done) => overlayLeave(el, done, 'pop')"
      >
        <div
          v-if="menuOpen"
          ref="popEl"
          class="menu-pop m-float"
          role="menu"
          :style="popStyle"
          tabindex="-1"
          @keydown="onPopKeydown"
          @pointerdown.stop
        >
          <button class="menu-item" type="button" role="menuitem" @click="pick('password')">
            <Icon name="ep:lock" :size="16" />
            <span>修改密码</span>
          </button>
          <button class="menu-item" type="button" role="menuitem" @click="pick('appearance')">
            <Icon name="ep:sunny" :size="16" />
            <span>外观设置</span>
          </button>
          <!-- components.css 无 .menu-divider，用现有描边色做 1px 分隔 -->
          <div aria-hidden="true" style="height: 1px; margin: 6px 8px; background: var(--glass-stroke-2)" />
          <button class="menu-item is-danger" type="button" role="menuitem" @click="pick('logout')">
            <Icon name="ep:circle-close" :size="16" />
            <span>退出登录</span>
          </button>
        </div>
      </Transition>
    </Teleport>
  </header>
</template>
