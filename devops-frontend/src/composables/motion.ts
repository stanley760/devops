/**
 * 玻璃动效系统 —— GSAP 统一入口（页面不得另写 GSAP 配方）。
 *
 * 每条动效对应一种工业能量语态：
 *   放 Act      —— 按压、涟漪、弹窗展开、浮条升起（瞬时释放）
 *   导 Trace    —— 折线描绘、刻度环扫掠、时间块生长、仪表指针推进
 *   结 Settle   —— 读数滚动落定、容量带生长、卡片落位
 *   停 Halt     —— 抽屉收回、拖拽取消飞回、浮条下沉
 *
 * 三层「减少动态」保险：
 *   1. skipMotion()（prefers-reduced-motion 或页面隐藏）时全部直达终态；
 *   2. visibilitychange 到 hidden 时强制进行中的补间 progress(1)；
 *   3. CSS 层 @media (prefers-reduced-motion: reduce) 收掉漂移层。
 */
import gsap from 'gsap'
import { Flip } from 'gsap/Flip'

gsap.registerPlugin(Flip)

export { gsap, Flip }

/* ---------------- 常量表 ---------------- */

export const EASE = Object.freeze({
  reveal: 'expo.out',
  glide: 'power4.out',
  exit: 'power2.in',
  swap: 'power2.inOut',
  settle: 'back.out(1.55)',
})

export const DUR = Object.freeze({
  press: 0.14,
  release: 0.46,
  hoverIn: 0.32,
  hoverOut: 0.4,
  slide: 0.52,
  pageOut: 0.18,
  pageIn: 0.62,
  overlayIn: 0.42,
  overlayOut: 0.2,
})

/* ---------------- 减少动态 ---------------- */

export function prefersReducedMotion(): boolean {
  return window.matchMedia('(prefers-reduced-motion: reduce)').matches
}

export function motionSkipped(): boolean {
  return prefersReducedMotion() || document.hidden
}

function skipMotion(): boolean {
  return motionSkipped()
}

// 页面隐藏时把进行中的补间直接推到终态，避免回来时半截动画
if (typeof document !== 'undefined') {
  document.addEventListener('visibilitychange', () => {
    if (document.hidden) {
      gsap.globalTimeline.getChildren(true, true, true).forEach((t) => {
        try {
          t.progress(1)
        } catch {
          /* 已销毁的补间忽略 */
        }
      })
    }
  })
}

/** 双 rAF：等待首帧栅格完成，避免 backdrop-filter 首帧卡顿。 */
export function afterTwoFrames(fn: () => void): void {
  requestAnimationFrame(() => requestAnimationFrame(fn))
}

/* ---------------- 全局委托微交互 ---------------- */

const PRESSABLE = [
  '.btn',
  '.icon-btn',
  '.nav-item',
  '.segmented button',
  '.pg-btn',
  '.svc-tile',
  '.cmd-trigger',
  '.user-chip',
  '.menu-item',
].join(',')

/** 放：按压反馈（只写 scale，与 hover 的 y 分通道不冲突）。 */
export function installPressFeedback(): void {
  document.addEventListener('pointerdown', (e) => {
    const el = (e.target as Element | null)?.closest?.(PRESSABLE)
    if (!el || (el as HTMLButtonElement).disabled) return
    gsap.to(el, { scale: 0.955, duration: DUR.press, ease: 'power2.out', overwrite: 'auto' })
    const up = () => {
      gsap.to(el, { scale: 1, duration: DUR.release, ease: EASE.settle, overwrite: 'auto' })
      window.removeEventListener('pointerup', up)
      window.removeEventListener('pointercancel', up)
    }
    window.addEventListener('pointerup', up)
    window.addEventListener('pointercancel', up)
  })
}

/** 结：悬停轻抬升（仅精确指针设备）。 */
export function installHoverLift(): void {
  if (!window.matchMedia('(hover: hover)').matches) return
  document.addEventListener('pointerover', (e) => {
    const el = (e.target as Element | null)?.closest?.('.lift')
    if (!el) return
    gsap.to(el, { y: -3, duration: DUR.hoverIn, ease: EASE.glide, overwrite: 'auto' })
  })
  document.addEventListener('pointerout', (e) => {
    const el = (e.target as Element | null)?.closest?.('.lift')
    if (!el) return
    gsap.to(el, { y: 0, duration: DUR.hoverOut, ease: EASE.swap, overwrite: 'auto' })
  })
}

/** 放：主按钮涟漪（从点击点化开）。 */
export function installRipple(): void {
  document.addEventListener('pointerdown', (e) => {
    const el = (e.target as Element | null)?.closest?.('.btn-primary, .btn-danger')
    if (!el || (el as HTMLButtonElement).disabled) return
    const rect = el.getBoundingClientRect()
    const size = Math.max(rect.width, rect.height) * 1.9
    const clip = document.createElement('span')
    clip.className = 'ripple-clip'
    const dot = document.createElement('span')
    dot.className = 'ripple-dot'
    dot.style.width = `${size}px`
    dot.style.height = `${size}px`
    dot.style.left = `${e.clientX - rect.left - size / 2}px`
    dot.style.top = `${e.clientY - rect.top - size / 2}px`
    clip.appendChild(dot)
    el.appendChild(clip)
    gsap.to(dot, {
      scale: 1,
      opacity: 0,
      duration: 0.62,
      ease: EASE.reveal,
      onComplete: () => clip.remove(),
    })
  })
}

/** 指针光：把指针位置写入容器 CSS 变量（材质氛围）。 */
export function trackGlow(event: PointerEvent): void {
  const el = (event.currentTarget as HTMLElement | null)?.closest?.('.m-glow') ?? (event.currentTarget as HTMLElement | null)?.querySelector?.('.m-glow')
  const target = (el as HTMLElement | null) ?? (event.currentTarget as HTMLElement | null)
  if (!target) return
  const rect = target.getBoundingClientRect()
  target.style.setProperty('--px', `${event.clientX - rect.left}px`)
  target.style.setProperty('--py', `${event.clientY - rect.top}px`)
}

/* ---------------- 页面与外壳入场 ---------------- */

/**
 * 跳过动画时调用过渡的 done()。
 *
 * **必须是异步的**：router-view 的 Transition 用了 `mode="out-in"`，它的状态机靠
 * done() 回调来触发「旧页面移除 → 新页面挂载」。若在 leave 钩子里同步调用 done()，
 *  Vue 还在当前 patch 流程中，不会接着渲染新页面，enter 钩子也就不再触发 ——
 * 结果是URL 变了、路由也匹配了，内容区却永远空白。
 * 开了「减少动态效果」（prefers-reduced-motion）的用户必踩这个坑，
 * 且没有任何报错，极难排查。推到宏任务末尾即可绕开。
 */
function settleDone(done: () => void): void {
  setTimeout(done, 0)
}

/** 导 / 结：页面进入（router-view Transition @enter）。 */
export function pageEnter(el: Element, done: () => void): void {
  if (skipMotion()) {
    ;(el as HTMLElement).classList.add('is-entrance-ready')
    settleDone(done)
    return
  }
  const reveals = el.querySelectorAll('.reveal')
  const bentos = el.querySelectorAll('.bento')
  gsap.set(el, { opacity: 0, y: 18 })
  const tl = gsap.timeline({ onComplete: done })
  tl.to(el, { opacity: 1, y: 0, duration: DUR.pageIn, ease: EASE.reveal }, 0)
  if (reveals.length) {
    tl.fromTo(
      reveals,
      { opacity: 0, y: 16 },
      { opacity: 1, y: 0, duration: 0.52, ease: EASE.reveal, stagger: 0.05 },
      0.06,
    )
  }
  if (bentos.length) {
    tl.fromTo(
      bentos,
      { opacity: 0, y: 22, scale: 0.97 },
      { opacity: 1, y: 0, scale: 1, duration: 0.58, ease: EASE.reveal, stagger: 0.045 },
      0.08,
    )
  }
  ;(el as HTMLElement).classList.add('is-entrance-ready')
}

/** 停：页面离开。 */
export function pageLeave(el: Element, done: () => void): void {
  if (skipMotion()) {
    settleDone(done)
    return
  }
  gsap.to(el, { opacity: 0, y: -10, duration: DUR.pageOut, ease: EASE.exit, onComplete: done })
}

/** 无过渡钩子时的首绘入场。 */
export function revealPage(page: Element): void {
  if (skipMotion()) return
  pageEnter(page, () => undefined)
}

/** 结：外壳首入场（品牌 → 导航组 → 顶栏）。 */
export function playShellEntrance(frame: Element): void {
  if (skipMotion()) return
  const tl = gsap.timeline({ defaults: { ease: EASE.reveal } })
  tl.fromTo(frame.querySelector('.rail-brand'), { opacity: 0, y: -12 }, { opacity: 1, y: 0, duration: 0.5 }, 0)
  tl.fromTo(
    frame.querySelectorAll('.nav-group, .nav-item'),
    { opacity: 0, x: -14 },
    { opacity: 1, x: 0, duration: 0.44, stagger: 0.035 },
    0.08,
  )
  tl.fromTo(
    frame.querySelectorAll('.topbar > *'),
    { opacity: 0, y: -10 },
    { opacity: 1, y: 0, duration: 0.42, stagger: 0.05 },
    0.18,
  )
}

/**
 * 导：共享玻璃块滑动（导航选中 / 分段滑块）。
 * 用几何差值定位并反解祖先 transform 缩放，弹窗缩放下也稳定。
 */
export function slideThumb(thumb: Element, targetSel: string, axis: 'x' | 'y', instant = false): void {
  const parent = thumb.parentElement
  const target = parent?.querySelector(targetSel) as HTMLElement | null
  if (!parent || !target) return

  const pRect = parent.getBoundingClientRect()
  const tRect = target.getBoundingClientRect()
  const thRect = thumb.getBoundingClientRect()
  // 反解祖先缩放
  const sx = pRect.width ? thRect.width / (thumb as HTMLElement).offsetWidth || 1 : 1
  const dx = (tRect.left - pRect.left) / (sx || 1)
  const dy = (tRect.top - pRect.top) / (sx || 1)

  thumb.classList.add('is-shown')
  if (instant || skipMotion()) {
    gsap.set(thumb, {
      x: axis === 'x' ? dx : 0,
      y: axis === 'y' ? dy : 0,
      width: tRect.width / (sx || 1),
      height: tRect.height / (sx || 1),
    })
    return
  }
  gsap.to(thumb, {
    x: axis === 'x' ? dx : 0,
    y: axis === 'y' ? dy : 0,
    width: tRect.width / (sx || 1),
    height: tRect.height / (sx || 1),
    duration: DUR.slide,
    ease: EASE.swap,
    overwrite: 'auto',
  })
}

/* ---------------- 指针原点（浮层从点击点展开） ---------------- */

export const lastPointer = {
  x: 0,
  y: 0,
  at: 0,
}

let pointerHooked = false
function ensurePointerTracking(): void {
  if (pointerHooked) return
  pointerHooked = true
  window.addEventListener(
    'pointerdown',
    (e) => {
      lastPointer.x = e.clientX
      lastPointer.y = e.clientY
      lastPointer.at = Date.now()
    },
    { capture: true },
  )
}

/** 返回浮层展开原点：600ms 内的点击点，否则视口中心。 */
function overlayOrigin(): { x: number; y: number } {
  ensurePointerTracking()
  if (Date.now() - lastPointer.at < 600) return { x: lastPointer.x, y: lastPointer.y }
  return { x: window.innerWidth / 2, y: window.innerHeight / 2 }
}

/* ---------------- 浮层过渡 ---------------- */

export type OverlayKind = 'modal' | 'drawer' | 'pop'

/** 放：浮层进入。modal 自指针点展开，drawer 自右侧滑入，pop 自下浮出。 */
export function overlayEnter(el: Element, done: () => void, kind: OverlayKind = 'modal'): void {
  const node = el as HTMLElement
  if (skipMotion()) {
    done()
    return
  }
  if (kind === 'drawer') {
    gsap.fromTo(
      node,
      { xPercent: 102, opacity: 0.6 },
      { xPercent: 0, opacity: 1, duration: DUR.overlayIn, ease: EASE.reveal, onComplete: done },
    )
    return
  }
  if (kind === 'pop') {
    gsap.fromTo(
      node,
      { opacity: 0, y: 8, scale: 0.96 },
      { opacity: 1, y: 0, scale: 1, duration: 0.26, ease: EASE.settle, onComplete: done },
    )
    return
  }
  const o = overlayOrigin()
  const rect = node.getBoundingClientRect()
  const ox = o.x - rect.left
  const oy = o.y - rect.top
  gsap.set(node, { transformOrigin: `${ox}px ${oy}px` })
  gsap.fromTo(
    node,
    { opacity: 0, scale: 0.86, y: 10 },
    { opacity: 1, scale: 1, y: 0, duration: DUR.overlayIn, ease: EASE.reveal, onComplete: done },
  )
}

/** 停：浮层离开。 */
export function overlayLeave(el: Element, done: () => void, kind: OverlayKind = 'modal'): void {
  const node = el as HTMLElement
  if (skipMotion()) {
    done()
    return
  }
  if (kind === 'drawer') {
    gsap.to(node, { xPercent: 102, opacity: 0.5, duration: DUR.overlayOut, ease: EASE.exit, onComplete: done })
    return
  }
  if (kind === 'pop') {
    gsap.to(node, { opacity: 0, y: 6, scale: 0.96, duration: 0.16, ease: EASE.exit, onComplete: done })
    return
  }
  gsap.to(node, { opacity: 0, scale: 0.92, duration: DUR.overlayOut, ease: EASE.exit, onComplete: done })
}

/** 放 / 停：底部批量操作浮条。 */
export function floatBarEnter(el: Element, done: () => void): void {
  if (skipMotion()) {
    done()
    return
  }
  gsap.fromTo(
    el,
    { opacity: 0, y: 34, scale: 0.96 },
    { opacity: 1, y: 0, scale: 1, duration: 0.4, ease: EASE.settle, onComplete: done },
  )
}

export function floatBarLeave(el: Element, done: () => void): void {
  if (skipMotion()) {
    done()
    return
  }
  gsap.to(el, { opacity: 0, y: 24, duration: 0.22, ease: EASE.exit, onComplete: done })
}

/* ---------------- 内容级联 ---------------- */

/** 结：列表行入场（最多 14 行）。 */
export function revealRows(rows: Element[] | NodeListOf<Element>): void {
  if (skipMotion()) return
  const list = Array.from(rows).slice(0, 14)
  if (!list.length) return
  gsap.fromTo(
    list,
    { opacity: 0, y: 12 },
    { opacity: 1, y: 0, duration: 0.4, ease: EASE.reveal, stagger: 0.035, overwrite: 'auto' },
  )
}

/** 结：面板内容级联（70–90ms 节奏，最多 18 项）。 */
export function revealCascade(container: Element, selector = ':scope > *', gap = 0.078): void {
  if (skipMotion()) return
  const items = Array.from(container.querySelectorAll(selector)).slice(0, 18)
  if (!items.length) return
  gsap.fromTo(
    items,
    { opacity: 0, y: 18 },
    { opacity: 1, y: 0, duration: 0.52, ease: EASE.reveal, stagger: gap, overwrite: 'auto' },
  )
}

/** 结：一次确认弹跳（提交成功等）。 */
export function settlePop(el: Element): void {
  if (skipMotion()) return
  gsap.fromTo(el, { scale: 0.92 }, { scale: 1, duration: 0.42, ease: EASE.settle, overwrite: 'auto' })
}

/* ---------------- 描绘 / 仪表 ---------------- */

/** 导：SVG 路径描绘。 */
export function drawPath(path: SVGPathElement | SVGElement, area?: SVGElement): void {
  const node = path as SVGPathElement
  const len = typeof node.getTotalLength === 'function' ? node.getTotalLength() : 0
  if (skipMotion() || !len) {
    if (area) gsap.set(area, { opacity: 1 })
    return
  }
  gsap.fromTo(
    node,
    { strokeDasharray: len, strokeDashoffset: len },
    { strokeDashoffset: 0, duration: 1.15, ease: EASE.reveal },
  )
  if (area) gsap.fromTo(area, { opacity: 0 }, { opacity: 1, duration: 0.9, delay: 0.25, ease: EASE.reveal })
}

/**
 * 导：刻度环 / 容量带扫掠到目标比例。
 * @param el 元素（读取 data-fill 或直接传 value）
 * @param value 0–1
 * @param setter 自定义属性写入（默认写 width 百分比）
 */
export function gaugeSweep(
  el: Element,
  value: number,
  setter?: (v: number) => void,
): void {
  const state = { v: 0 }
  const write = setter ?? ((v: number) => {
    ;(el as HTMLElement).style.setProperty('--fill', `${(v * 100).toFixed(1)}%`)
  })
  if (skipMotion()) {
    write(value)
    return
  }
  gsap.to(state, {
    v: value,
    duration: 1.05,
    ease: EASE.reveal,
    onUpdate: () => write(state.v),
  })
}

/** 结：读数滚动落定（工业数字滚轮）。 */
export function countUp(
  el: HTMLElement,
  target: number,
  opts: { decimals?: number; duration?: number; prefix?: string; suffix?: string } = {},
): void {
  const decimals = opts.decimals ?? 0
  const fmt = (v: number) =>
    `${opts.prefix ?? ''}${v.toLocaleString('zh-CN', {
      minimumFractionDigits: decimals,
      maximumFractionDigits: decimals,
    })}${opts.suffix ?? ''}`
  if (skipMotion()) {
    el.textContent = fmt(target)
    return
  }
  const state = { v: Number(el.dataset.value ?? 0) }
  el.dataset.value = String(target)
  gsap.to(state, {
    v: target,
    duration: opts.duration ?? 0.85,
    ease: EASE.reveal,
    onUpdate: () => {
      el.textContent = fmt(state.v)
    },
  })
}

/* ---------------- 物理 / 几何 ---------------- */

export interface SpringState {
  x: number
  v: number
}

export function springOf(x = 0): SpringState {
  return { x, v: 0 }
}

/**
 * 结：临界阻尼弹簧积分（k=62 ≈ 0.5s 落位）。
 * c = 2*sqrt(k) 时无过冲。
 */
export function spring(state: SpringState, target: number, dt: number, k = 62): void {
  const c = 2 * Math.sqrt(k)
  const a = k * (target - state.x) - c * state.v
  state.v += a * dt
  state.x += state.v * dt
}

/** 三次贝塞尔取点（管线连线用）。 */
export function bezierAt(p0: number, p1: number, p2: number, p3: number, u: number): number {
  const m = 1 - u
  return m * m * m * p0 + 3 * m * m * u * p1 + 3 * m * u * u * p2 + u * u * u * p3
}

export interface FrameLoop {
  start(el?: Element): void
  stop(): void
  flush(): void
  readonly time: number
}

/** 常驻帧循环：离屏 / 页面隐藏 / 减少动态自动暂停；每帧 try/catch 保活。 */
export function createFrameLoop(step: (dt: number, t: number) => void): FrameLoop {
  let running = false
  let last = 0
  let t = 0
  let frames = 0
  let watched: Element | null = null
  let inView = true
  let errLogged = false
  let tickFn: (() => void) | null = null

  const measure = () => {
    if (!watched) {
      inView = true
      return
    }
    const r = watched.getBoundingClientRect()
    inView = r.bottom > -80 && r.top < window.innerHeight + 80
  }

  return {
    start(el?: Element) {
      watched = el ?? null
      if (running) return
      running = true
      last = performance.now()
      measure()
      tickFn = () => {
        if (!running) return
        const now = performance.now()
        let dt = (now - last) / 1000
        last = now
        if (dt > 1 / 30) dt = 1 / 30
        if (document.hidden || motionSkipped() || !inView) return
        t += dt
        if (++frames % 20 === 0) measure()
        try {
          step(dt, t)
        } catch (e) {
          if (!errLogged) {
            errLogged = true
            console.warn('[motion] frame loop error', e)
          }
        }
      }
      gsap.ticker.add(tickFn)
    },
    stop() {
      running = false
      if (tickFn) {
        gsap.ticker.remove(tickFn)
        tickFn = null
      }
    },
    flush() {
      measure()
    },
    get time() {
      return t
    },
  }
}

/** 导：打字机（问答 / 终端回显），返回取消函数。 */
export function typeOut(text: string, write: (v: string) => void, done?: () => void, speed = 42): () => void {
  if (skipMotion()) {
    write(text)
    done?.()
    return () => undefined
  }
  let i = 0
  const timer = window.setInterval(() => {
    i += 1
    write(text.slice(0, i))
    if (i >= text.length) {
      window.clearInterval(timer)
      done?.()
    }
  }, speed)
  return () => window.clearInterval(timer)
}

/** 会话内一次性入场（刷新后不再播）。 */
export function firstTimeInSession(key: string): boolean {
  const k = `devops.intro.${key}`
  if (sessionStorage.getItem(k)) return false
  sessionStorage.setItem(k, '1')
  return true
}

/** Flip 共享布局形变。 */
export async function flipLayout(targets: Element[] | NodeListOf<Element>, mutate: () => void | Promise<void>): Promise<void> {
  const list = Array.from(targets)
  if (skipMotion() || !list.length) {
    await mutate()
    return
  }
  const state = Flip.getState(list)
  await mutate()
  Flip.from(state, {
    duration: 0.52,
    ease: EASE.swap,
    absolute: true,
    stagger: 0.02,
    nested: true,
  })
}
