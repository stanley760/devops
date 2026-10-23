<script setup lang="ts">
/**
 * 左侧导航轨：品牌 / 分组菜单 / 滑块高亮 / 版本脚注。
 * 菜单数据来自 auth 动态导航（navGroups），点击路由跳转并通知外壳收起移动端抽屉。
 */
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { navGroups } from '@/composables/auth'
import { slideThumb } from '@/composables/motion'
import Icon from '@/components/Icon.vue'

const emit = defineEmits<{ navigate: [] }>()

const route = useRoute()
const router = useRouter()

const navEl = ref<HTMLElement | null>(null)
const thumbEl = ref<HTMLElement | null>(null)
let ro: ResizeObserver | null = null

/** 导：滑块对准当前激活项（找不到激活项时保持原位）。 */
function syncThumb(): void {
  const thumb = thumbEl.value
  if (!thumb) return
  if (!navEl.value?.querySelector('.nav-item.is-active')) return
  slideThumb(thumb, '.nav-item.is-active', 'y')
}

function go(path: string): void {
  emit('navigate')
  if (route.path === path) return
  void router.push(path)
}

onMounted(() => {
  void nextTick(syncThumb)
  ro = new ResizeObserver(() => syncThumb())
  if (navEl.value) ro.observe(navEl.value)
})

onBeforeUnmount(() => {
  ro?.disconnect()
  ro = null
})

watch(navGroups, () => {
  void nextTick(syncThumb)
})
watch(
  () => route.path,
  () => {
    void nextTick(syncThumb)
  },
)
</script>

<template>
  <aside class="rail m-rail">
    <div class="rail-brand">
      <div class="brand-mark">
        <Icon name="ep:compass" :size="18" />
      </div>
      <div class="brand-text">
        <div class="brand-name">DevOps 管理平台</div>
        <div class="brand-sub">OPS CONSOLE</div>
      </div>
    </div>

    <nav class="rail-nav" ref="navEl" aria-label="主导航">
      <div class="nav-thumb m-pill" ref="thumbEl" aria-hidden="true" />
      <div class="nav-group" v-for="group in navGroups" :key="group.key">
        <div v-if="!group.single" class="nav-group-title">{{ group.label }}</div>
        <button
          v-for="item in group.items"
          :key="item.key"
          class="nav-item"
          :class="{ 'is-active': route.path === item.path }"
          :aria-current="route.path === item.path ? 'page' : undefined"
          type="button"
          @click="go(item.path)"
        >
          <span class="nav-icon">
            <Icon v-if="item.icon" :name="item.icon" :size="18" />
          </span>
          <span class="nav-label">{{ item.label }}</span>
        </button>
      </div>
    </nav>

    <div class="rail-foot">v0.1.0 · titanium-glass</div>
  </aside>
</template>
