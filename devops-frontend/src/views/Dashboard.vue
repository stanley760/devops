<template>
  <div ref="pageRef" class="wb">
    <!-- 主视觉：欢迎 + 租户铭牌 -->
    <section class="wb-hero reveal">
      <div class="eyebrow hero-eyebrow">WORKBENCH · 工作台</div>
      <div class="hero-hello">{{ greeting }}，{{ displayName }}</div>
      <p class="hero-line">
        当前工作区租户为「{{ tenantName }}」。左侧菜单来自角色授权的实时菜单树，
        页面数据全部读取 devops-system 接口；快速入口直达各管理模块。
      </p>
      <div class="hero-meta">
        <span class="tag tag-blue">租户 {{ tenantCode }}</span>
        <span class="tag tag-violet">角色 × {{ roles.length || 1 }}</span>
        <span class="tag tag-ok">权限点 {{ permissions.length }}</span>
        <span v-if="currentUser?.loginDate" class="tag">
          上次登录 {{ formatDateTime(currentUser.loginDate) }}
        </span>
      </div>
    </section>

    <div class="wb-grid" style="margin-top: 16px">
      <div class="wb-col">
        <!-- 快捷入口（真实菜单） -->
        <Panel class="reveal" title="快捷入口" :meta="`${entries.length} 个可用模块`">
          <div class="wb-quick">
            <button
              v-for="e in entries"
              :key="e.path"
              class="wb-entry lift"
              type="button"
              @click="go(e.path)"
            >
              <span class="entry-icon"><Icon :name="e.icon || 'ep:grid'" :size="16" /></span>
              <span class="entry-label">{{ e.label }}</span>
              <span class="entry-path">{{ e.path }}</span>
            </button>
            <div v-if="!entries.length" class="empty" style="grid-column: 1 / -1">
              <div class="empty-icon"><Icon name="ep:folder" :size="22" /></div>
              <div>暂无已授权的管理菜单，请联系租户管理员分配角色。</div>
            </div>
          </div>
        </Panel>

        <!-- 最近登录（真实日志） -->
        <Panel v-if="canLoginLog" class="reveal" title="最近登录" meta="登录日志 · 最近 6 条">
          <div>
            <div v-for="row in loginRows" :key="row.id" class="wb-log-row">
              <StatusTag
                :kind="row.result === 0 ? 'ok' : 'crit'"
                :text="row.result === 0 ? '成功' : '失败'"
                :led="false"
              />
              <span class="log-main mono">{{ row.username }}</span>
              <span class="log-main" style="flex: 1">{{ row.ip || '—' }} · {{ formatLoginLocation(row) }}</span>
              <span class="log-time">{{ formatTimeOfDay(row.createTime) }}</span>
            </div>
            <div v-if="!loginRows.length" class="empty">
              <div class="empty-icon"><Icon name="ep:document" :size="22" /></div>
              <div>暂无登录记录</div>
            </div>
          </div>
        </Panel>
      </div>

      <div class="wb-col">
        <!-- 账号铭牌 -->
        <Panel class="reveal" title="当前账号">
          <div class="wb-account">
            <Avatar :name="displayName" :size="52" />
            <div>
              <div class="account-name">{{ displayName }}</div>
              <div class="account-sub">{{ currentUser?.username || '—' }} · {{ currentUser?.id || '' }}</div>
            </div>
          </div>
          <SpecPlate
            style="margin-top: 14px"
            light
            title="ACCOUNT / TENANT"
            :rows="accountRows"
          />
        </Panel>

        <!-- 真实统计（仅接口返回的计数） -->
        <Panel class="reveal" title="资源计数" meta="来自系统接口">
          <div class="wb-stats">
            <div class="wb-mini">
              <span class="mini-label">菜单入口</span>
              <span ref="menuNum" class="mini-value">0</span>
            </div>
            <div class="wb-mini">
              <span class="mini-label">登录日志</span>
              <span ref="loginNum" class="mini-value">0</span>
            </div>
            <div class="wb-mini">
              <span class="mini-label">操作日志</span>
              <span ref="operNum" class="mini-value">0</span>
            </div>
          </div>
        </Panel>

        <!-- 最近操作（真实日志） -->
        <Panel v-if="canOperLog" class="reveal" title="最近操作" meta="操作日志 · 最近 6 条">
          <div>
            <div v-for="row in operRows" :key="row.id" class="wb-log-row">
              <span class="log-main">{{ row.title || row.action || '操作' }}</span>
              <span class="log-time">{{ formatTimeOfDay(row.operTime) }}</span>
            </div>
            <div v-if="!operRows.length" class="empty">
              <div class="empty-icon"><Icon name="ep:document" :size="22" /></div>
              <div>暂无操作记录</div>
            </div>
          </div>
        </Panel>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
/**
 * 工作台：只呈现真实接口数据（账号 / 租户 / 菜单 / 审计日志），
 * 不虚构监控指标；菜单集合与 /auth/routers 一致。
 */
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import Panel from '@/components/ui/Panel.vue'
import Avatar from '@/components/ui/Avatar.vue'
import SpecPlate from '@/components/ui/SpecPlate.vue'
import StatusTag from '@/components/ui/StatusTag.vue'
import Icon from '@/components/Icon.vue'
import { currentUser, currentTenant, hasPermission, navGroups, permissions, roles } from '@/composables/auth'
import { countUp, revealCascade } from '@/composables/motion'
import { formatDateTime, formatTimeOfDay } from '@/composables/datetime'
import { loginLogApi, operationLogApi, type LoginLog, type OperationLog } from '@/api/system'

const router = useRouter()
const pageRef = ref<HTMLElement | null>(null)
const menuNum = ref<HTMLElement | null>(null)
const loginNum = ref<HTMLElement | null>(null)
const operNum = ref<HTMLElement | null>(null)

const loginRows = ref<LoginLog[]>([])
const operRows = ref<OperationLog[]>([])
const loginTotal = ref(0)
const operTotal = ref(0)

const canLoginLog = computed(() => hasPermission('system:loginlog:list'))
const canOperLog = computed(() => hasPermission('system:operationlog:list'))

const displayName = computed(
  () => currentUser.value?.nickname || currentUser.value?.username || '同事',
)
const tenantName = computed(() => currentTenant.value?.name || 'default')
const tenantCode = computed(() => currentTenant.value?.code || 'default')

const greeting = computed(() => {
  const h = new Date().getHours()
  if (h < 6) return '夜深了'
  if (h < 11) return '早上好'
  if (h < 14) return '中午好'
  if (h < 18) return '下午好'
  return '晚上好'
})

/** 快捷入口 = 授权菜单树（跳过工作台自身） */
const entries = computed(() =>
  navGroups.value
    .flatMap((g) => g.items)
    .filter((item) => item.path !== '/')
    .slice(0, 12),
)

const accountRows = computed(() => [
  { key: '租户', val: `${tenantName.value}（${tenantCode.value}）` },
  { key: '部门', val: currentUser.value?.deptId || '—' },
  { key: '角色', val: roles.value.join(' / ') || '—' },
  { key: '邮箱', val: currentUser.value?.email || '—' },
  { key: '手机', val: currentUser.value?.mobile || '—' },
])

function go(path: string): void {
  void router.push(path)
}

function formatLoginLocation(row: LoginLog): string {
  return (
    [row.geoCountry, row.geoProvince, row.geoCity, row.geoIsp].filter(Boolean).join(' / ') || '内网'
  )
}

onMounted(() => {
  revealCascade(pageRef.value ?? document.body, '.reveal', 0.09)

  if (canLoginLog.value) {
    loginLogApi
      .list({ page_num: 1, page_size: 6 })
      .then((res) => {
        loginRows.value = res.list ?? []
        loginTotal.value = res.total ?? 0
        if (loginNum.value) countUp(loginNum.value, loginTotal.value, { duration: 1.1 })
      })
      .catch(() => undefined)
  }
  if (canOperLog.value) {
    operationLogApi
      .list({ page_num: 1, page_size: 6 })
      .then((res) => {
        operRows.value = res.list ?? []
        operTotal.value = res.total ?? 0
        if (operNum.value) countUp(operNum.value, operTotal.value, { duration: 1.1 })
      })
      .catch(() => undefined)
  }
  if (menuNum.value) {
    countUp(menuNum.value, entries.value.length, { duration: 1 })
  }
})
</script>
