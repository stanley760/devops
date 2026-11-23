import { useCallback, useEffect, useRef, useState } from 'react'
import { PortalDialog } from '@/portal/components/PortalDialog'
import { Button } from '@/components/ui/button'
import type { CaptchaAnswer, CaptchaChallenge } from '@/lib/types'

interface Props {
  open: boolean
  challenge: CaptchaChallenge | null
  onAnswer: (answer: CaptchaAnswer) => void
  onRefresh: () => void
  onCancel: () => void
}

/**
 * 拖动旋转时，拖过一整张图的宽度对应转过的角度。
 *
 * 服务端把缩略图按 block.Angle 顺时针转过，主图未转（block.Angle 取值 30~330），
 * 用户要把缩略图转回正向，需要再转 360 - block.Angle 度。
 *
 * 前端拿不到 block.Angle（答案在服务端），所以按"拖过一整张图的宽度 = 转一整圈"
 * 来映射角度（与官方 go-captcha-vue 旋转组件同一思路）：拖动负责大范围，
 * 滑杆负责最后几度的微调。提交的就是当前施加在缩略图上的 CSS 旋转角，
 * 缩略图看上去与主图对齐时，该角度正好是 360 - block.Angle。
 */
const ROTATE_FULL_TURN_DEG = 360

/** 把角度收进 [0, 360)。 */
function normalizeAngle(angle: number): number {
  return ((Math.round(angle) % 360) + 360) % 360
}

/**
 * 显示尺寸与图片像素尺寸的比例。
 *
 * 挑战图按 challenge.width 渲染，但容器可能被 CSS 缩放（窄屏），
 * 拖动位移必须换算回图片像素空间，否则提交的坐标会偏。
 * jsdom 里没有布局（宽度为 0），按 1:1 处理。
 */
function displayScale(rect: DOMRect | undefined, natural: number): number {
  if (!rect || rect.width <= 0) return 1
  return natural / rect.width
}

function clamp(value: number, min: number, max: number): number {
  if (max < min) return min
  return Math.min(Math.max(value, min), max)
}

// portal 侧四题型验证码弹窗：click 点选 / slide+dragdrop 拖拽拼图 / rotate 旋转。
// 挑战一次性，校验失败由调用方重新拉题并复用本组件（onRefresh → 新 challenge.id 即重置交互）。
export function PortalCaptchaDialog({ open, challenge, onAnswer, onRefresh, onCancel }: Props) {
  const [points, setPoints] = useState<Array<{ x: number; y: number }>>([])
  const [offset, setOffset] = useState({ x: 0, y: 0 })
  const [angle, setAngle] = useState(0)
  const [dragging, setDragging] = useState(false)

  // 拖动过程中的基准值：起点指针位置、起点的偏移/角度。
  // 用 ref 保存，move 事件读到的一定是本次拖动的基准，
  // 不会因为 React 状态更新滞后而把上一次的偏移带进来。
  const dragRef = useRef<{
    mode: 'piece' | 'rotate'
    startX: number
    startY: number
    baseOffset: { x: number; y: number }
    baseAngle: number
  } | null>(null)

  const containerRef = useRef<HTMLDivElement | null>(null)
  // 拖动回调要读最新的挑战（题型决定是否允许纵向拖动），但它注册在 window 上，
  // 用 ref 读最新的挑战，避免每次挑战变化都重新挂监听。
  const challengeRef = useRef<CaptchaChallenge | null>(challenge)
  challengeRef.current = challenge
  // 同样的道理：拖动开始时的偏移/角度也要是最新的值。
  const offsetRef = useRef(offset)
  const angleRef = useRef(angle)
  offsetRef.current = offset
  angleRef.current = angle

  useEffect(() => {
    if (open) {
      setPoints([])
      setOffset({ x: 0, y: 0 })
      setAngle(0)
    }
  }, [open, challenge?.id])

  const submit = useCallback(() => {
    if (!challenge) return
    switch (challenge.type) {
      case 'click':
        onAnswer({ points })
        break
      case 'slide':
      case 'dragdrop':
        onAnswer({
          x: (challenge.thumb_x ?? 0) + offset.x,
          y: (challenge.thumb_y ?? 0) + offset.y,
        })
        break
      case 'rotate':
        onAnswer({ angle })
        break
    }
  }, [challenge, points, offset, angle, onAnswer])

  const onImageClick = (e: React.MouseEvent<HTMLImageElement>) => {
    if (challenge?.type !== 'click') return
    const rect = e.currentTarget.getBoundingClientRect()
    const scaleX = displayScale(rect, challenge.width)
    const scaleY = displayScale(rect, challenge.height)
    setPoints((p) => [
      ...p,
      {
        x: Math.round((e.clientX - rect.left) * scaleX),
        y: Math.round((e.clientY - rect.top) * scaleY),
      },
    ])
  }

  /** 拖动开始：记下指针位置与本次拖动的基准偏移/角度。 */
  const beginDrag = useCallback((mode: 'piece' | 'rotate', clientX: number, clientY: number) => {
    dragRef.current = {
      mode,
      startX: clientX,
      startY: clientY,
      baseOffset: { ...offsetRef.current },
      baseAngle: angleRef.current,
    }
    setDragging(true)
  }, [])

  /**
   * 拖动中：位移以本次拖动的起点为基准累计（而不是只用上一帧的增量），
   * 旋转角度同理，所以拖出多少角度就是多少角度。
   */
  const moveDrag = useCallback((clientX: number, clientY: number) => {
    const drag = dragRef.current
    const current = challengeRef.current
    if (!drag || !current) return
    const rect = containerRef.current?.getBoundingClientRect()
    const scale = displayScale(rect, current.width)

    if (drag.mode === 'rotate') {
      // 拖过一整张图的宽度 = 转一整圈（ROTATE_FULL_TURN_DEG 度）。
      const displayWidth = rect && rect.width > 0 ? rect.width : current.width
      const degPerPx = displayWidth > 0 ? ROTATE_FULL_TURN_DEG / displayWidth : 0
      const deg = drag.baseAngle + (clientX - drag.startX) * degPerPx
      setAngle(normalizeAngle(deg))
      return
    }

    const thumbW = current.thumb_width ?? 0
    const thumbH = current.thumb_height ?? 0
    const baseX = current.thumb_x ?? 0
    const baseY = current.thumb_y ?? 0
    let nextX = baseX + drag.baseOffset.x + (clientX - drag.startX) * scale
    let nextY = baseY + drag.baseOffset.y + (clientY - drag.startY) * scale

    // slide 只有横向一轴：拼图块始终停在切图所在的那一行，
    // 后端按 slide.Validate 以 5px 的容差校验 y，纵向漂移会直接判失败。
    if (current.type === 'slide') nextY = baseY

    // 钳制到图片范围内：拼图块不能被拖出图片，
    // 否则提交的落点坐标落在图外，后端按 cutout 位置比较必然不匹配。
    nextX = clamp(nextX, 0, Math.max(0, current.width - thumbW))
    nextY = clamp(nextY, 0, Math.max(0, current.height - thumbH))

    setOffset({ x: nextX - baseX, y: nextY - baseY })
  }, [])
  const endDrag = useCallback(() => {
    dragRef.current = null
    setDragging(false)
  }, [])

  // 拖动期间监听 window：指针离开图片范围时拖动仍然继续，松手才结束。
  useEffect(() => {
    if (!dragging) return
    const onMouseMove = (e: MouseEvent) => moveDrag(e.clientX, e.clientY)
    const onMouseUp = () => endDrag()
    const onTouchMove = (e: TouchEvent) => {
      const pt = e.touches[0] ?? e.changedTouches[0]
      if (!pt) return
      if (e.cancelable) e.preventDefault()
      moveDrag(pt.clientX, pt.clientY)
    }
    const onTouchEnd = () => endDrag()
    window.addEventListener('mousemove', onMouseMove)
    window.addEventListener('mouseup', onMouseUp)
    window.addEventListener('touchmove', onTouchMove, { passive: false })
    window.addEventListener('touchend', onTouchEnd)
    window.addEventListener('touchcancel', onTouchEnd)
    return () => {
      window.removeEventListener('mousemove', onMouseMove)
      window.removeEventListener('mouseup', onMouseUp)
      window.removeEventListener('touchmove', onTouchMove)
      window.removeEventListener('touchend', onTouchEnd)
      window.removeEventListener('touchcancel', onTouchEnd)
    }
  }, [dragging, moveDrag, endDrag])

  if (!challenge) return null

  const imgW = challenge.width
  const imgH = challenge.height
  const isDrag = challenge.type === 'slide' || challenge.type === 'dragdrop'

  const onPiecePointerDown = (e: React.MouseEvent | React.TouchEvent) => {
    const pt = 'touches' in e ? e.touches[0] : e
    if (!pt) return
    beginDrag('piece', pt.clientX, pt.clientY)
  }

  const onRotatePointerDown = (e: React.MouseEvent | React.TouchEvent) => {
    const pt = 'touches' in e ? e.touches[0] : e
    if (!pt) return
    beginDrag('rotate', pt.clientX, pt.clientY)
  }

  return (
    <PortalDialog open={open} onOpenChange={(o) => { if (!o) onCancel() }} title="安全验证">
      <p className="text-muted-foreground mb-2 text-sm">
        {challenge.type === 'click' && `请依次点击：${challenge.text ?? ''}`}
        {challenge.type === 'slide' && '拖动滑块完成拼图'}
        {challenge.type === 'dragdrop' && '将拼图块拖到正确位置'}
        {challenge.type === 'rotate' && '拖动或用滑杆旋转图片至正向'}
      </p>
      <div
        ref={containerRef}
        className="relative select-none"
        style={{ width: imgW, maxWidth: '100%', aspectRatio: `${imgW} / ${imgH}` }}
      >
        <img src={challenge.image} alt="captcha" className="w-full rounded-md" onClick={onImageClick} />
        {isDrag && challenge.thumb && (
          <img
            src={challenge.thumb}
            alt="puzzle piece"
            className="absolute cursor-grab"
            style={{
              // 用百分比定位：容器被 CSS 缩放时（窄屏），拼图块跟着图片一起缩放，
              // 不会与图片内容错位。
              left: `${(((challenge.thumb_x ?? 0) + offset.x) / imgW) * 100}%`,
              top: `${(((challenge.thumb_y ?? 0) + offset.y) / imgH) * 100}%`,
              width: `${((challenge.thumb_width ?? 0) / imgW) * 100}%`,
              height: `${((challenge.thumb_height ?? 0) / imgH) * 100}%`,
            }}
            onMouseDown={onPiecePointerDown}
            onTouchStart={onPiecePointerDown}
          />
        )}
        {challenge.type === 'click' &&
          points.map((p, i) => (
            <span
              key={i}
              className="bg-primary pointer-events-none absolute h-4 w-4 -translate-x-1/2 -translate-y-1/2 rounded-full opacity-70"
              style={{ left: `${(p.x / imgW) * 100}%`, top: `${(p.y / imgH) * 100}%` }}
            />
          ))}
      </div>
      {challenge.type === 'rotate' && challenge.thumb && (
        <div className="flex flex-col items-center gap-2">
          <img
            src={challenge.thumb}
            alt="rotate target"
            style={{ width: challenge.thumb_size ?? 120, transform: `rotate(${angle}deg)` }}
            className="cursor-grab touch-none rounded-md"
            onMouseDown={onRotatePointerDown}
            onTouchStart={onRotatePointerDown}
          />
          <input
            type="range"
            min={0}
            max={360}
            step={1}
            value={angle}
            onChange={(e) => setAngle(normalizeAngle(Number(e.target.value)))}
            className="w-full"
            aria-label="旋转角度"
          />
        </div>
      )}
      <div className="mt-3 flex gap-2">
        <Button variant="outline" className="glass-control flex-1" onClick={onRefresh}>换一题</Button>
        <Button className="flex-1" onClick={submit}>提交</Button>
      </div>
    </PortalDialog>
  )
}
