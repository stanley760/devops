<template>
  <div class="login-stage deep-stage">
    <!-- 左：深色拓扑画布（品牌视觉） -->
    <section class="login-canvas">
      <div class="canvas-art">
        <TopoNet :nodes="30" :glow="1" />
        <div class="canvas-glow"></div>
      </div>

      <header class="login-brand reveal">
        <span class="brand-mark"><Icon name="ep:compass" :size="22" /></span>
        <span class="brand-text">
          <span class="brand-name">DevOps 钛玻璃控制台</span>
          <span class="brand-sub">TITANIUM GLASS · OPS CONSOLE</span>
        </span>
      </header>

      <div class="login-hero">
        <div class="eyebrow hero-eyebrow reveal">MULTI-TENANT IAM PLATFORM</div>
        <h1 class="hero-title reveal">
          工业级运维<br />
          从一枚玻璃开始
        </h1>
        <p class="hero-sub reveal">
          多租户用户权限中台：JWT 双令牌会话、Casbin RBAC 域模型、
          菜单按钮级授权、单库租户隔离。登录后按角色菜单进入管理模块。
        </p>
        <div class="hero-tags reveal">
          <span class="hero-tag">JWT DUAL-TOKEN</span>
          <span class="hero-tag">RBAC + DOMAIN</span>
          <span class="hero-tag">TENANT ISOLATION</span>
          <span class="hero-tag">SNOWFLAKE ID</span>
        </div>
      </div>

      <footer class="login-foot reveal">
        <span>VERSION v0.1.0</span>
        <i class="foot-sep"></i>
        <span>{{ clock }}</span>
        <i class="foot-sep"></i>
        <span>API :9999 /system</span>
      </footer>
    </section>

    <!-- 右：钛玻璃登录卡 -->
    <section class="login-side">
      <form
        ref="cardRef"
        class="login-card m-float m-glow reveal"
        novalidate
        @submit.prevent="openCaptchaModal"
      >
        <div class="glow-layer"></div>
        <h2 class="login-title">登录</h2>
        <p class="login-desc">使用租户内的账号进入控制台</p>

        <div class="login-form">
          <!-- 租户不出现在页面上：未登录访问时守卫已把默认租户拼进地址
               （/login?tenantCode=default），脚本静默解析用于提交登录；
               更换租户请改地址栏 tenantCode 的值 -->
          <Field label="账号" :error="errors.username">
            <TextInput v-model="form.username" placeholder="admin" icon="ep:user" />
          </Field>
          <Field label="密码" :error="errors.password">
            <TextInput v-model="form.password" type="password" placeholder="••••••••" icon="ep:lock" />
          </Field>

          <div class="login-row">
            <Checkbox v-model="remember" label="记住账号" />
          </div>

          <p v-if="loginError" class="login-error" role="alert">{{ loginError }}</p>

          <button class="btn btn-primary btn-block" type="submit" :disabled="busy || captchaOpen">
            <span v-if="!done">{{ busy ? '请稍候…' : '登 录' }}</span>
            <span v-else class="done-mark">✓</span>
          </button>
        </div>

        <p class="login-tip">
          管理员 <code>admin / Admin@123</code><br />
          点击登录后需完成安全验证；连续失败会触发限流。
        </p>
      </form>
    </section>

    <!-- 验证码弹框：点「登录」后才向后端请求题目，验证通过立即提交登录并跳转 -->
    <transition name="captcha-modal">
      <div v-if="captchaOpen" class="captcha-mask" @click.self="closeCaptchaModal">
        <div class="captcha-modal m-float m-glow" role="dialog" aria-modal="true" aria-label="安全验证">
          <div class="glow-layer"></div>

          <header class="captcha-modal-head">
            <div class="captcha-modal-heading">
              <h3>安全验证</h3>
              <p>完成验证后自动登录</p>
            </div>
            <button
              class="captcha-modal-close"
              type="button"
              aria-label="关闭验证"
              :disabled="submitting"
              @click="closeCaptchaModal"
            >
              <Icon name="ep:close" :size="18" />
            </button>
          </header>

          <div class="captcha-modal-body">
            <p v-if="captchaLoading" class="captcha-modal-state">正在加载验证码…</p>
            <template v-else>
              <div ref="captchaHostRef" class="captcha-host">
                <component
                  :is="captchaComponent"
                  :key="challenge?.id ?? 'none'"
                  :config="captchaConfig"
                  :data="captchaData"
                  :events="captchaEvents"
                />
              </div>
              <p class="captcha-note" :class="{ 'is-ok': submitting, 'is-err': !!captchaModalError }">
                {{ captchaModalError || (submitting ? '正在登录…' : captchaPrompt) }}
              </p>
            </template>
          </div>

          <footer class="captcha-modal-foot">
            <button class="btn btn-ghost btn-block" type="button" :disabled="submitting" @click="closeCaptchaModal">
              取消
            </button>
          </footer>
        </div>
      </div>
    </transition>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  Click as GocaptchaClick,
  Rotate as GocaptchaRotate,
  Slide as GocaptchaSlide,
  SlideRegion as GocaptchaSlideRegion,
} from 'go-captcha-vue'
import 'go-captcha-vue/dist/style.css'
import TopoNet from '@/components/TopoNet.vue'
import Icon from '@/components/Icon.vue'
import Field from '@/components/ui/Field.vue'
import TextInput from '@/components/ui/TextInput.vue'
import Checkbox from '@/components/ui/Checkbox.vue'
import { authApi, solveCaptcha, type CaptchaAnswer, type CaptchaChallenge } from '@/api/auth'
import { ApiError } from '@/api/http'
import { homePath, login } from '@/composables/auth'
import { rememberTenant, resolveTenantEntry } from '@/composables/tenantEntry'
import { gsap, EASE, afterTwoFrames, firstTimeInSession, trackGlow } from '@/composables/motion'
import { toastOk } from '@/composables/toast'

const router = useRouter()
const route = useRoute()

const cardRef = ref<HTMLFormElement | null>(null)
const captchaHostRef = ref<HTMLDivElement | null>(null)

/* ---------------- 租户解析（地址优先，页面零展示） ----------------
 *
 * 租户只从地址来（见 composables/tenantEntry.ts）：
 *   - ?tenantCode=xxx（默认入口：未登录访问任意页都会被守卫带回 /login?tenantCode=…，
 *     首页即 tenantCode=default——默认值由守卫拼在地址里，页面无需任何输入或展示）
 *   - /t/{code}/login（分享链接）、{code}.admin.example.com（通配域名）
 *   - 都没有 → 记住的上次租户 → default。
 *
 * 换租户就是改地址栏 tenantCode 的值。解析出的编码只用于提交登录，
 * 登录框因此只剩账号 + 密码。
 *
 * 用 watch 而不是 setup 里取一次：/login 与 /t/:tenantCode/login 共用本组件，
 * 路由间跳转会复用实例，参数变了必须跟着重解析，否则租户会残留成上一个入口的。
 */

const tenantEntry = ref(
  resolveTenantEntry(
    typeof route.params.tenantCode === 'string' ? route.params.tenantCode : null,
    typeof route.query.tenantCode === 'string' ? route.query.tenantCode : null,
  ),
)

const form = reactive({
  tenant_code: tenantEntry.value.code,
  username: localStorage.getItem('devops.last_user') || '',
  password: '',
})

watch(
  () => [route.params.tenantCode, route.query.tenantCode],
  () => {
    tenantEntry.value = resolveTenantEntry(
      typeof route.params.tenantCode === 'string' ? route.params.tenantCode : null,
      typeof route.query.tenantCode === 'string' ? route.query.tenantCode : null,
    )
    form.tenant_code = tenantEntry.value.code
  },
)
const errors = reactive({ username: '', password: '' })
const remember = ref(true)
const busy = ref(false)
const done = ref(false)

/* ---------------- 租户选择（已移除） ----------------
 *
 * 这里曾经是「可登录租户下拉 + 文本输入兜底」，现已整段删除：
 * 租户统一由地址携带（/login?tenantCode=xxx，未登录访问任意页时守卫自动补上），
 * 页面再给选择/输入入口只会让人登进错误的组织，也和「改地址栏即可换租户」重复。
 * 登录框因此只剩账号 + 密码；后端 /auth/tenants 接口也已一并删除。
 */

/* ---------------- 验证码 ----------------
 *
 * 四种题型共用官方 go-captcha-vue 组件，题目类型由后端下发（t_system_config: sys.captcha.type），
 * 前端只负责把 challenge 映射成组件的 config / data，并在 confirm 时回传答案。
 *
 * 交互模式：验证码**不常驻登录页**。进页面不请求题目；用户点「登录」时才拉一道题并弹框，
 * 验证通过立即提交登录，成功后跳转控制台。这样一次就过的用户完全不用接触验证码。
 *
 * 坐标语义（与服务端 captcha.Verify 一一对应，无需任何换算）：
 *   - click:    组件给出的点已是主图像素坐标（内部用 getBoundingClientRect + pageX 求差），
 *               主图按 config.width/height 1:1 渲染，与后端 Dot.X/Y 同一像素空间；
 *   - slide:    confirm 回传 { x: 拼图块左上角绝对 x, y: data.thumbY }，与后端 block.X/Y（缺口左上角）同语义；
 *   - dragdrop: 同上，二维自由拖动；
 *   - rotate:   confirm 回传用户施加的 CSS 顺时针角度，服务端校验 sub.Angle + block.Angle ≈ 360。
 *     故 data.angle 必须下发 0（初始角），绝不能下发 block.Angle —— 那等于泄露答案，
 *     且当 block.Angle > 180 时目标角会落在可达区间 [0,360] 之外，题目将无解。
 */

const challenge = ref<CaptchaChallenge | null>(null)

/** 已作答的答案；未完成时为 null，用于提交前拦截。 */
const solved = ref<CaptchaAnswer | null>(null)

/** 弹框是否打开（打开时禁用登录按钮，避免重复拉题）。 */
const captchaOpen = ref(false)

/** 弹框内正在换题（官方组件右上角刷新图标触发）。 */
const captchaLoading = ref(false)

/** 正在提交登录（验证已通过，等待服务端响应）。 */
const submitting = ref(false)

/** 弹框内的错误提示（验证失败 / 换题失败 / 未点选就确认）。 */
const captchaModalError = ref('')

/**
 * 登录卡上的表单级错误（密码错、租户不存在、限流…）。
 *
 * 不用 toast：ToastHost 只挂在 AppShell 里，登录页弹的 toast 根本不会渲染，
 * 用户会看到「点了登录什么都没发生」。所以这类错误必须落在登录卡自身的 DOM 上。
 */
const loginError = ref('')

/** 组件实例类型（官方包的具名导出），按题型挑选。 */
const captchaComponent = computed(() => {
  switch (challenge.value?.type) {
    case 'click':
      return GocaptchaClick
    case 'slide':
      return GocaptchaSlide
    case 'dragdrop':
      return GocaptchaSlideRegion
    default:
      return GocaptchaRotate
  }
})

/** 主图尺寸固定 1:1 渲染，这是点击坐标能直接用的前提。 */
const MAIN_WIDTH = 300
const MAIN_HEIGHT = 220

const captchaConfig = computed(() => {
  const ch = challenge.value
  const base = {
    width: ch?.width || MAIN_WIDTH,
    height: ch?.height || MAIN_HEIGHT,
    // 深色登录页上用官方主题包裹，靠 --go-captcha-theme-* 覆写配色（见 pages.css）
    showTheme: true,
  }
  if (ch?.type === 'click') {
    return { ...base, dotSize: 24, buttonText: '确认', title: clickTitle(ch.text) }
  }
  if (ch?.type === 'rotate') {
    // size 是圆形裁剪区边长，需与主图边长一致
    return { ...base, size: ch.width || MAIN_HEIGHT, scope: true }
  }
  return { ...base, scope: true }
})

const captchaData = computed(() => {
  const ch = challenge.value
  if (!ch) return {}
  if (ch.type === 'click') {
    return { image: ch.image, thumb: ch.thumb || '' }
  }
  if (ch.type === 'rotate') {
    // angle 固定 0：见上方 rotate 语义说明
    return { image: ch.image, thumb: ch.thumb || '', angle: 0, thumbSize: ch.thumb_size || 0 }
  }
  return {
    image: ch.image,
    thumb: ch.thumb || '',
    thumbX: ch.thumb_x || 0,
    thumbY: ch.thumb_y || 0,
    thumbWidth: ch.thumb_width || 0,
    thumbHeight: ch.thumb_height || 0,
  }
})

const captchaEvents = computed(() => ({
  // 官方组件右上角刷新图标：换一道题。用户主动换题时清掉上一轮的错误提示
  refresh: () => {
    captchaModalError.value = ''
    void refreshCaptcha()
  },
  confirm: (payload: unknown) => {
    const ch = challenge.value
    if (!ch) return
    if (ch.type === 'click') {
      const dots = payload as Array<{ x: number; y: number }>
      if (!dots?.length) {
        // 官方按钮在没点选时也可点，这里给出明确反馈而不是静默无反应
        captchaModalError.value = '请先按顺序点选图形，再点「确认」'
        return
      }
      solved.value = solveCaptcha(ch, { points: dots.map((d) => ({ x: d.x, y: d.y })) })
    } else if (ch.type === 'rotate') {
      solved.value = solveCaptcha(ch, { angle: payload as number })
    } else {
      const p = payload as { x: number; y: number }
      solved.value = solveCaptcha(ch, { x: p.x, y: p.y })
    }
    captchaModalError.value = ''
    // 验证通过即提交登录，不需要用户再点一次「登录」
    void submitLogin()
  },
}))

const captchaPrompt = computed(() => {
  const ch = challenge.value
  if (!ch) return ''
  if (ch.type === 'click') return '按顺序点选图形后点「确认」，通过后自动登录'
  if (ch.type === 'rotate') return '拖动滑块旋转缩略图，使缺口朝向正上方后松手'
  return '拖动滑块把拼图块移到缺口位置，松手即校验'
})

/** 把后端下发的英文形状名翻成中文提示（后端 text 形如 "circle,star"）。 */
function clickTitle(text?: string): string {
  if (!text) return '请在下图依次点击'
  const labels: Record<string, string> = {
    circle: '圆形',
    square: '方形',
    triangle: '三角形',
    diamond: '菱形',
    star: '星形',
  }
  const names = text
    .split(',')
    .map((s) => labels[s.trim()] ?? s.trim())
    .filter(Boolean)
  return names.length ? `请依次点击：${names.join('、')}` : '请在下图依次点击'
}

/**
 * 点「登录」的第一步：拉一道验证码并弹出验证框。
 *
 * 为什么不在进页面时就拉：验证码常驻登录页会把「先做题再登录」变成默认流程，
 * 而大多数用户一次就能登录成功；放到点击登录后，只有真正要登录的人才需要解题。
 *
 * 后端未开启验证码时（enabled=false）直接提交登录，不弹框 —— 否则用户还得解一道
 * 服务端根本不会校验的题。
 */
async function openCaptchaModal(): Promise<void> {
  if (captchaOpen.value || submitting.value) return
  errors.username = form.username.trim() ? '' : '请输入账号'
  errors.password = form.password ? '' : '请输入密码'
  if (errors.username || errors.password) return
  // 理论上不会为空（解析链兜底到 default）；但租户为空时后端定位不了租户配置，仍拦一下
  if (!form.tenant_code.trim()) {
    loginError.value = '租户缺失：请通过地址栏携带 tenantCode 参数（如 ?tenantCode=default）'
    return
  }

  busy.value = true
  captchaModalError.value = ''
  loginError.value = ''
  try {
    const ch = await authApi.captcha(form.tenant_code.trim() || undefined)
    if (!ch || ch.enabled === false) {
      challenge.value = null
      solved.value = null
      await submitLogin()
      return
    }
    challenge.value = ch
    solved.value = null
    captchaOpen.value = true
    // 等组件把题目渲染出来（DOM 更新是异步的），再修正确认按钮的 type
    await nextTick()
    fixCaptchaButtonType()
  } catch (e) {
    // 同 submitLogin：登录页没有 ToastHost，错误显示在卡片上
    loginError.value = loginErrorMessage(e)
  } finally {
    busy.value = false
  }
}

/**
 * 换一道新题，弹框保持打开。
 *
 * 刻意**不清空** `captchaModalError`：答错后会自动调它换题，而那时错误提示必须留着，
 * 否则用户刚看到「验证码错误」就被打回默认文案，等于没提示。清错误由调用方决定 ——
 * 用户主动点刷新图标时才该清（见 captchaEvents.refresh）。
 */
async function refreshCaptcha(): Promise<void> {
  if (captchaLoading.value || submitting.value) return
  captchaLoading.value = true
  solved.value = null
  try {
    challenge.value = await authApi.captcha(form.tenant_code.trim() || undefined)
  } catch {
    captchaModalError.value = '加载失败，请点右上角图标重试'
  } finally {
    captchaLoading.value = false
  }
}

/** 关闭弹框，放弃本次登录尝试（题目未消费，重新点登录会拿新题）。 */
function closeCaptchaModal(): void {
  if (submitting.value) return
  captchaOpen.value = false
  captchaModalError.value = ''
  challenge.value = null
  solved.value = null
}

/* ---------------- 提交 ---------------- */

/**
 * 提交登录。captcha 为 undefined 表示后端未开启验证码。
 *
 * 失败要分两种，处理方式完全不同：
 *
 *   - **验证码答错**（服务端 code 400）：弹框保持打开，在框内显示错误并**自动换一道新题**。
 *     挑战是一次性消费的（服务端 GetDel），答错即作废，所以必须换新题；但没必要把用户
 *     踢回表单再点一次登录 —— 他本来就盯着验证码，直接重试才是顺的。
 *   - **其他失败**（密码错 401、租户不存在、限流 429…）：关弹框回到表单，错误显示在登录卡上。
 *     这些要靠改表单内容解决，而表单在弹框后面，不关掉用户没法改。
 *
 * 用错误码区分而不是匹配文案：400 = 参数/验证码错误，401 = 凭据错误，是后端既有的稳定约定。
 */
async function submitLogin(): Promise<void> {
  if (submitting.value) return
  submitting.value = true
  try {
    if (remember.value) {
      localStorage.setItem('devops.last_user', form.username.trim())
      // 记一下：下次访问没带 tenantCode 时，兜底链落到这个租户
      rememberTenant(form.tenant_code)
    }
    await login({
      tenant_code: form.tenant_code.trim() || undefined,
      username: form.username.trim(),
      password: form.password,
      captcha: solved.value ?? undefined,
    })
    captchaOpen.value = false
    toastOk('登录成功')
    playSuccess()
    await new Promise((r) => window.setTimeout(r, 520))
    const redirect = route.query.redirect
    await router.replace(typeof redirect === 'string' && redirect ? redirect : homePath())
  } catch (e) {
    // finally 会再置一次 false，这里先置是为了让 refreshCaptcha 的守卫放行
    submitting.value = false
    if (isCaptchaError(e)) {
      // 先换题再显示错误：refreshCaptcha 会进入 loading 态，错误文案在它结束后出现，
      // 时序上不会互相覆盖
      void refreshCaptcha()
      captchaModalError.value = captchaErrorMessage(e)
      return
    }
    captchaOpen.value = false
    challenge.value = null
    solved.value = null
    // 登录页没有挂 ToastHost，toast 在这里不会渲染，所以错误直接显示在卡片上
    loginError.value = loginErrorMessage(e)
  } finally {
    submitting.value = false
  }
}

/** 验证码相关失败：后端用 CodeParam(400) 表示参数/验证码错误。 */
function isCaptchaError(e: unknown): boolean {
  return e instanceof ApiError && e.code === 400
}

/** 把服务端文案翻成中文提示；认不出来时退回原文，避免显示空提示。 */
function captchaErrorMessage(e: unknown): string {
  const msg = e instanceof Error ? e.message : ''
  return /captcha/i.test(msg) || !msg ? '验证码错误，请重新验证' : msg
}

/**
 * 登录失败的友好文案。
 *
 * 服务端返回的是英文（incorrect username or password 等），登录页是中文界面，
 * 直接甩英文给用户很难看。这里做一层展示映射；认不出来的退回原文，至少不会空白。
 */
const LOGIN_ERROR_TEXT: Array<[RegExp, string]> = [
  [/incorrect username or password/i, '账号或密码错误'],
  [/user does not exist/i, '账号不存在或已被删除'],
  [/tenant not found/i, '租户不存在，请重新选择'],
  [/tenant is disabled/i, '租户已被禁用，请联系管理员'],
  [/too many|rate limit/i, '操作过于频繁，请稍后再试'],
]

function loginErrorMessage(e: unknown): string {
  const msg = e instanceof Error ? e.message : ''
  for (const [re, text] of LOGIN_ERROR_TEXT) {
    if (re.test(msg)) return text
  }
  return msg || '登录失败，请重试'
}

function playSuccess(): void {
  done.value = true
  const card = cardRef.value
  if (card && !matchMedia('(prefers-reduced-motion: reduce)').matches) {
    gsap.to(card, { scale: 0.94, opacity: 0, duration: 0.5, ease: EASE.reveal })
  }
}

/* ---------------- 时钟 / 入场 / 指针倾斜 ---------------- */

const clock = ref('')
let clockTimer = 0

function tickClock(): void {
  clock.value = new Date().toLocaleTimeString('zh-CN', { hour12: false })
}

/**
 * 指针倾斜：卡片随指针做 3D 微倾。
 *
 * 为什么遇到控件就停：倾斜是 transform 3D，会让卡内元素在指针移动时发生位移。
 * 登录卡宽 400px、倾角 ±6°，实测「确认」按钮在指针移动过程中会漂移约 22px
 * （超过按钮高度一半），用户瞄准的是他看到的按钮，按下时按钮已经移开，
 * 点击落到别处，表现为「点了没反应」。控件需要精确点击，倾斜在这里只有副作用。
 *
 * 注意是「停在当前角度」而不是「复位到 0」：复位本身也是一次位移，同样会让按钮
 * 在用户瞄准期间跑掉。停住不动才是可点击的状态。
 */
const NO_TILT_SELECTOR = 'input, textarea, select, button, a, label, .captcha-host'

/** 倾斜补间的 id，用来在需要停住时精确干掉它（不影响入场动画那条补间）。 */
const TILT_TWEEN_ID = 'login-card-tilt'

function onMove(e: PointerEvent): void {
  const card = cardRef.value
  if (!card || matchMedia('(prefers-reduced-motion: reduce)').matches) return
  trackGlow(e)

  const target = e.target as Element | null
  if (target?.closest?.(NO_TILT_SELECTOR)) {
    gsap.getById(TILT_TWEEN_ID)?.kill()
    return
  }

  const rect = card.getBoundingClientRect()
  const dx = (e.clientX - rect.left) / rect.width - 0.5
  const dy = (e.clientY - rect.top) / rect.height - 0.5
  gsap.to(card, {
    id: TILT_TWEEN_ID,
    rotationY: dx * 6,
    rotationX: -dy * 6,
    transformPerspective: 900,
    duration: 0.5,
    ease: EASE.glide,
    overwrite: 'auto',
  })
}

/**
 * 官方组件的「确认」按钮是裸 <button>，没有 type 属性。
 *
 * 放在我们的 <form> 里，浏览器会把它当成 type="submit"：点一下确认，既记录了
 * 验证码答案，又立刻触发表单提交去登录。表现是「点确认后直接跳到登录失败」，
 * 用户完全搞不清发生了什么。这里把 type 显式改成 button，让确认只做它该做的事。
 *
 * 组件是异步渲染的（题目数据到达后才 mount），所以用 MutationObserver 持续修正 ——
 * 弹框内换题会重建组件节点，观察器挂在不会重建的 .captcha-host 上，能一并兜住。
 */
function fixCaptchaButtonType(): void {
  const host = captchaHostRef.value
  if (!host) return
  const patch = (): void => {
    host.querySelectorAll<HTMLButtonElement>('button').forEach((b) => {
      if (b.type !== 'button') b.type = 'button'
    })
  }
  patch()
  captchaObserver?.disconnect()
  const obs = new MutationObserver(patch)
  obs.observe(host, { childList: true, subtree: true })
  captchaObserver = obs
}

let captchaObserver: MutationObserver | null = null

function onLeave(): void {
  const card = cardRef.value
  if (!card) return
  gsap.getById(TILT_TWEEN_ID)?.kill()
  gsap.to(card, { rotationX: 0, rotationY: 0, duration: 0.7, ease: EASE.settle, overwrite: 'auto' })
}

/** Esc 关闭验证弹框（与点遮罩/取消一致）。 */
function onKeydown(e: KeyboardEvent): void {
  if (e.key === 'Escape' && captchaOpen.value) closeCaptchaModal()
}

onMounted(() => {
  tickClock()
  clockTimer = window.setInterval(tickClock, 1000)

  const card = cardRef.value
  if (card) {
    card.addEventListener('pointermove', onMove)
    card.addEventListener('pointerleave', onLeave)
  }
  window.addEventListener('keydown', onKeydown)

  afterTwoFrames(() => {
    if (!firstTimeInSession('login-intro')) return
    const reveals = document.querySelectorAll('.login-stage .reveal')
    gsap.fromTo(
      reveals,
      { opacity: 0, y: 22 },
      { opacity: 1, y: 0, duration: 0.62, ease: EASE.reveal, stagger: 0.07 },
    )
  })
})

onBeforeUnmount(() => {
  window.clearInterval(clockTimer)
  window.removeEventListener('keydown', onKeydown)
  captchaObserver?.disconnect()
  captchaObserver = null
  const card = cardRef.value
  card?.removeEventListener('pointermove', onMove)
  card?.removeEventListener('pointerleave', onLeave)
})
</script>

<style scoped>
.canvas-glow {
  position: absolute;
  inset: 0;
  background:
    radial-gradient(720px 420px at 24% 26%, rgba(255, 255, 255, 0.055), transparent 62%),
    radial-gradient(560px 380px at 82% 78%, rgba(255, 255, 255, 0.04), transparent 60%);
  pointer-events: none;
}

.captcha-note {
  margin: 6px 0 0;
  font-size: 11px;
  line-height: 1.5;
  color: var(--ink-3);
}

.captcha-note.is-ok {
  color: var(--ok-ink);
}

.captcha-note.is-err {
  color: var(--crit-ink);
}

/* 登录卡上的表单级错误（密码错 / 限流等）。
   登录页没有挂 ToastHost，toast 在这里不渲染，所以错误必须落在卡片自身的 DOM 上。 */
.login-error {
  margin: -2px 0 0;
  padding: 8px 11px;
  font-size: 12px;
  line-height: 1.5;
  color: var(--crit-ink);
  background: var(--crit-soft);
  border: 1px solid var(--crit);
  border-radius: var(--r-s);
}

/* ---------------- 验证码弹框 ---------------- */

.captcha-mask {
  position: fixed;
  inset: 0;
  z-index: 60;
  display: grid;
  place-items: center;
  padding: 24px;
  background: rgba(10, 10, 12, 0.62);
  -webkit-backdrop-filter: blur(6px);
  backdrop-filter: blur(6px);
}

.captcha-modal {
  width: 100%;
  max-width: 380px;
  padding: 20px 20px 18px;
  border-radius: var(--r-xl);
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.captcha-modal-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.captcha-modal-heading h3 {
  font-size: 17px;
  font-weight: 650;
  letter-spacing: -0.01em;
}

.captcha-modal-heading p {
  margin-top: 3px;
  font-size: 12px;
  color: var(--ink-3);
}

.captcha-modal-close {
  flex: none;
  width: 30px;
  height: 30px;
  display: grid;
  place-items: center;
  border-radius: 50%;
  color: var(--ink-3);
  background: var(--glass-2);
  transition:
    color 0.18s var(--ease-premium),
    background 0.18s var(--ease-premium);
}

.captcha-modal-close:hover {
  color: var(--ink);
  background: var(--glass-3);
}

.captcha-modal-body {
  display: flex;
  flex-direction: column;
}

.captcha-modal-state {
  padding: 48px 0;
  text-align: center;
  font-size: 12px;
  color: var(--ink-3);
}

.captcha-modal-foot {
  padding-top: 2px;
}

/* 进出场：遮罩淡入，面板轻微上浮 */
.captcha-modal-enter-active,
.captcha-modal-leave-active {
  transition: opacity 0.24s var(--ease-premium);
}

.captcha-modal-enter-from,
.captcha-modal-leave-to {
  opacity: 0;
}

.captcha-modal-enter-active .captcha-modal,
.captcha-modal-leave-active .captcha-modal {
  transition:
    transform 0.28s var(--ease-premium),
    opacity 0.24s var(--ease-premium);
}

.captcha-modal-enter-from .captcha-modal,
.captcha-modal-leave-to .captcha-modal {
  transform: translateY(14px) scale(0.97);
  opacity: 0;
}

.done-mark {
  font-size: 18px;
}
</style>
