<template>
  <div ref="host" class="topo-stage">
    <canvas ref="canvas"></canvas>
  </div>
</template>

<script setup lang="ts">
/**
 * 登录画布拓扑装饰：节点 + 连线 + 往返脉冲（纯品牌视觉，不承载监控数据）。
 * 常驻帧动画走 createFrameLoop（页面隐藏 / 离屏 / 减少动态自动暂停）。
 */
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { createFrameLoop, prefersReducedMotion } from '@/composables/motion'

const props = withDefaults(
  defineProps<{
    /** 节点数量（密度） */
    nodes?: number
    /** 脉冲亮度 0-1 */
    glow?: number
  }>(),
  { nodes: 26, glow: 1 },
)

const host = ref<HTMLDivElement | null>(null)
const canvas = ref<HTMLCanvasElement | null>(null)

interface NetNode {
  x: number
  y: number
  vx: number
  vy: number
  r: number
  phase: number
}

interface Pulse {
  a: number
  b: number
  t: number
  speed: number
}

let loop: ReturnType<typeof createFrameLoop> | null = null
let nodes: NetNode[] = []
let edges: Array<[number, number]> = []
let pulses: Pulse[] = []
let w = 0
let h = 0
let dpr = 1

function seed(): void {
  nodes = Array.from({ length: props.nodes }, () => ({
    x: Math.random() * w,
    y: Math.random() * h,
    vx: (Math.random() - 0.5) * 7,
    vy: (Math.random() - 0.5) * 7,
    r: 1.6 + Math.random() * 2.6,
    phase: Math.random() * Math.PI * 2,
  }))
  edges = []
  for (let i = 0; i < nodes.length; i++) {
    for (let j = i + 1; j < nodes.length; j++) {
      const dx = nodes[i].x - nodes[j].x
      const dy = nodes[i].y - nodes[j].y
      if (dx * dx + dy * dy < (Math.min(w, h) * 0.24) ** 2) edges.push([i, j])
    }
  }
  pulses = edges
    .filter(() => Math.random() > 0.72)
    .map(([a, b]) => ({ a, b, t: Math.random(), speed: 0.12 + Math.random() * 0.22 }))
}

function resize(): void {
  const el = host.value
  const c = canvas.value
  if (!el || !c) return
  const rect = el.getBoundingClientRect()
  dpr = Math.min(window.devicePixelRatio || 1, 2)
  w = rect.width
  h = rect.height
  c.width = Math.round(w * dpr)
  c.height = Math.round(h * dpr)
  seed()
}

function draw(dt: number, t: number): void {
  const c = canvas.value
  if (!c) return
  const ctx = c.getContext('2d')
  if (!ctx) return
  ctx.save()
  ctx.scale(dpr, dpr)
  ctx.clearRect(0, 0, w, h)

  // 连线
  for (const [i, j] of edges) {
    const a = nodes[i]
    const b = nodes[j]
    const dist = Math.hypot(a.x - b.x, a.y - b.y)
    const alpha = Math.max(0.03, 0.16 - dist / 2600)
    ctx.strokeStyle = `rgba(210, 212, 216, ${alpha})`
    ctx.lineWidth = 1
    ctx.beginPath()
    ctx.moveTo(a.x, a.y)
    ctx.lineTo(b.x, b.y)
    ctx.stroke()
  }

  // 脉冲
  for (const p of pulses) {
    p.t += dt * p.speed
    if (p.t > 1) {
      p.t = 0
      const swap = p.a
      p.a = p.b
      p.b = swap
    }
    const a = nodes[p.a]
    const b = nodes[p.b]
    const x = a.x + (b.x - a.x) * p.t
    const y = a.y + (b.y - a.y) * p.t
    const g = ctx.createRadialGradient(x, y, 0, x, y, 12 * props.glow)
    g.addColorStop(0, 'rgba(240, 240, 242, 0.85)')
    g.addColorStop(1, 'rgba(240, 240, 242, 0)')
    ctx.fillStyle = g
    ctx.beginPath()
    ctx.arc(x, y, 12 * props.glow, 0, Math.PI * 2)
    ctx.fill()
    ctx.fillStyle = 'rgba(252, 252, 253, 0.95)'
    ctx.beginPath()
    ctx.arc(x, y, 1.8, 0, Math.PI * 2)
    ctx.fill()
  }

  // 节点（缓慢呼吸）
  for (const n of nodes) {
    n.x += n.vx * dt
    n.y += n.vy * dt
    if (n.x < 0 || n.x > w) n.vx *= -1
    if (n.y < 0 || n.y > h) n.vy *= -1
    const breath = 0.55 + 0.45 * Math.sin(t * 0.9 + n.phase)
    ctx.fillStyle = `rgba(226, 227, 230, ${0.28 + breath * 0.5})`
    ctx.beginPath()
    ctx.arc(n.x, n.y, n.r, 0, Math.PI * 2)
    ctx.fill()
  }

  ctx.restore()
}

function drawStatic(): void {
  draw(0, 1.2)
}

onMounted(() => {
  resize()
  const ro = new ResizeObserver(() => {
    resize()
    drawStatic()
  })
  if (host.value) ro.observe(host.value)
  onBeforeUnmount(() => ro.disconnect())

  if (prefersReducedMotion()) {
    drawStatic()
    return
  }
  loop = createFrameLoop(draw)
  loop.start(host.value ?? undefined)
})

onBeforeUnmount(() => {
  loop?.stop()
  loop = null
})
</script>
