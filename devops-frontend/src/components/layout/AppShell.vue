<script setup lang="ts">
/**
 * 应用外壳：导航轨 + 顶栏 + 页面过渡 + 全局浮层宿主。
 * 会话清理 / 视觉偏好 / 动效均走 composables，本层只做编排。
 */
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import Sidebar from './Sidebar.vue'
import Topbar from './Topbar.vue'
import CommandPalette from './CommandPalette.vue'
import AppearanceDialog from './AppearanceDialog.vue'
import PasswordDialog from './PasswordDialog.vue'
import { ToastHost, ConfirmHost } from '@/components/ui'
import { appearance, toggleRail } from '@/composables/appearance'
import { logout } from '@/composables/auth'
import { confirmAction } from '@/composables/confirm'
import { pageEnter, pageLeave, playShellEntrance } from '@/composables/motion'

const route = useRoute()
const router = useRouter()

const frameRef = ref<HTMLElement | null>(null)
const contentRef = ref<HTMLElement | null>(null)
const mobileOpen = ref(false)
const scrolled = ref(false)

const cmdOpen = ref(false)
const appearOpen = ref(false)
const pwdOpen = ref(false)

const mqMobile = window.matchMedia('(max-width: 820px)')

function onMqChange(): void {
  if (!mqMobile.matches) mobileOpen.value = false
}

function handleToggleRail(): void {
  if (mqMobile.matches) mobileOpen.value = !mobileOpen.value
  else toggleRail()
}

function onContentScroll(): void {
  scrolled.value = (contentRef.value?.scrollTop ?? 0) > 8
}

function scrollToTop(): void {
  contentRef.value?.scrollTo({ top: 0 })
}

/** ⌘/Ctrl+K 开合命令面板。 */
function onGlobalKeydown(e: KeyboardEvent): void {
  if ((e.metaKey || e.ctrlKey) && !e.shiftKey && !e.altKey && e.key.toLowerCase() === 'k') {
    e.preventDefault()
    cmdOpen.value = !cmdOpen.value
  }
}

async function handleLogout(): Promise<void> {
  const ok = await confirmAction({
    title: '退出登录',
    text: '确认退出当前账号？未保存的操作将丢失。',
    danger: true,
    confirmLabel: '退出',
  })
  if (!ok) return
  await logout()
  await router.replace('/login')
}

onMounted(() => {
  if (frameRef.value) playShellEntrance(frameRef.value)
  mqMobile.addEventListener('change', onMqChange)
  window.addEventListener('keydown', onGlobalKeydown)
})

onBeforeUnmount(() => {
  mqMobile.removeEventListener('change', onMqChange)
  window.removeEventListener('keydown', onGlobalKeydown)
})
</script>

<template>
  <div class="stage grid-plate">
    <div
      ref="frameRef"
      class="app-frame"
      :class="{ 'is-collapsed': appearance.railCollapsed, 'is-mobile-open': mobileOpen }"
    >
      <Sidebar @navigate="mobileOpen = false" />
      <div class="rail-scrim" @click="mobileOpen = false" />

      <div class="workspace" :class="{ 'is-scrolled': scrolled }">
        <Topbar
          @toggle-rail="handleToggleRail"
          @command="cmdOpen = true"
          @appearance="appearOpen = true"
          @password="pwdOpen = true"
          @logout="handleLogout"
        />
        <main class="content" ref="contentRef" @scroll="onContentScroll">
          <div class="content-inner">
            <router-view v-slot="{ Component }">
              <Transition
                :css="false"
                mode="out-in"
                @leave="pageLeave"
                @enter="pageEnter"
                @after-leave="scrollToTop"
              >
                <div :key="route.path">
                  <component :is="Component" />
                </div>
              </Transition>
            </router-view>
          </div>
        </main>
      </div>
    </div>

    <CommandPalette :open="cmdOpen" @close="cmdOpen = false" />
    <AppearanceDialog :open="appearOpen" @close="appearOpen = false" />
    <PasswordDialog :open="pwdOpen" @close="pwdOpen = false" />

    <!-- 提示与确认宿主常驻，供全局轻交互使用 -->
    <ToastHost />
    <ConfirmHost />
  </div>
</template>
