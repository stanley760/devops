import { useCallback, useEffect, useRef, useState } from 'react'
import { Button } from '@/components/ui/button'
import {
  Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle,
} from '@/components/ui/dialog'
import type { CaptchaAnswer, CaptchaChallenge } from '@/lib/types'

interface CaptchaDialogProps {
  open: boolean
  challenge: CaptchaChallenge | null
  onAnswer: (answer: CaptchaAnswer) => void
  onRefresh: () => void
  onCancel: () => void
}

const ROTATE_FULL_TURN_DEG = 360

/** 把角度收进 [0, 360)。 */
function normalizeAngle(angle: number): number {
  return ((Math.round(angle) % 360) + 360) % 360
}

/** 显示尺寸与图片像素尺寸的比例；jsdom 无布局（宽度 0）时按 1:1 处理。 */
function displayScale(rect: DOMRect | undefined, natural: number): number {
  if (!rect || rect.width <= 0) return 1
  return natural / rect.width
}

function clamp(value: number, min: number, max: number): number {
  if (max < min) return min
  return Math.min(Math.max(value, min), max)
}

// 四题型统一交互组件：click 点击取点 / slide+dragdrop 拖拽拼图 / rotate 拖拽旋转
export function CaptchaDialog({ open, challenge, onAnswer, onRefresh, onCancel }: CaptchaDialogProps) {
  const [points, setPoints] = useState<Array<{ x: number; y: number }>>([])
  const [offset, setOffset] = useState({ x: 0, y: 0 })
  const [angle, setAngle] = useState(0)
  const [dragging, setDragging] = useState(false)

  // 拖动基准：起点指针位置与起点的偏移/角度，用 ref 保证 move 读到本次拖动的基准值。
  const dragRef = useRef<{
    mode: 'piece' | 'rotate'
    startX: number
    startY: number
    baseOffset: { x: number; y: number }
    baseAngle: number
  } | null>(null)
  const containerRef = useRef<HTMLDivElement | null>(null)
  const challengeRef = useRef<CaptchaChallenge | null>(challenge)
  challengeRef.current = challenge
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
      case 'dragdrop': {
        const bx = challenge.thumb_x ?? 0
        const by = challenge.thumb_y ?? 0
        onAnswer({ x: bx + offset.x, y: by + offset.y })
        break
      }
      case 'rotate':
        onAnswer({ angle })
        break
    }
  }, [challenge, points, offset, angle, onAnswer])

  const onImageClick = (e: React.MouseEvent<HTMLImageElement>) => {
    if (!challenge || challenge.type !== 'click') return
    const rect = e.currentTarget.getBoundingClientRect()
    const scaleX = displayScale(rect, challenge.width)
    const scaleY = displayScale(rect, challenge.height)
    setPoints((p) => [
      ...p,
      { x: Math.round((e.clientX - rect.left) * scaleX), y: Math.round((e.clientY - rect.top) * scaleY) },
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

  /** 位移以本次拖动的起点为基准累计，并将屏幕位移换算回图片像素空间。 */
  const moveDrag = useCallback((clientX: number, clientY: number) => {
    const drag = dragRef.current
    const current = challengeRef.current
    if (!drag || !current) return
    const rect = containerRef.current?.getBoundingClientRect()
    const scale = displayScale(rect, current.width)

    if (drag.mode === 'rotate') {
      const displayWidth = rect && rect.width > 0 ? rect.width : current.width
      const degPerPx = displayWidth > 0 ? ROTATE_FULL_TURN_DEG / displayWidth : 0
      setAngle(normalizeAngle(drag.baseAngle + (clientX - drag.startX) * degPerPx))
      return
    }

    const thumbW = current.thumb_width ?? 0
    const thumbH = current.thumb_height ?? 0
    const baseX = current.thumb_x ?? 0
    const baseY = current.thumb_y ?? 0
    let nextX = baseX + drag.baseOffset.x + (clientX - drag.startX) * scale
    let nextY = baseY + drag.baseOffset.y + (clientY - drag.startY) * scale

    // slide 只有横向一轴：拼图块始终停在切图所在的那一行。
    if (current.type === 'slide') nextY = baseY

    nextX = clamp(nextX, 0, Math.max(0, current.width - thumbW))
    nextY = clamp(nextY, 0, Math.max(0, current.height - thumbH))
    setOffset({ x: nextX - baseX, y: nextY - baseY })
  }, [])

  const endDrag = useCallback(() => {
    dragRef.current = null
    setDragging(false)
  }, [])

  // 拖动期间监听 window：指针离开图片范围时拖动仍继续，松手才结束。
  useEffect(() => {
    if (!dragging) return
    const onMouseMove = (e: MouseEvent) => moveDrag(e.clientX, e.clientY)
    const onTouchMove = (e: TouchEvent) => {
      const pt = e.touches[0] ?? e.changedTouches[0]
      if (!pt) return
      if (e.cancelable) e.preventDefault()
      moveDrag(pt.clientX, pt.clientY)
    }
    window.addEventListener('mousemove', onMouseMove)
    window.addEventListener('mouseup', endDrag)
    window.addEventListener('touchmove', onTouchMove, { passive: false })
    window.addEventListener('touchend', endDrag)
    window.addEventListener('touchcancel', endDrag)
    return () => {
      window.removeEventListener('mousemove', onMouseMove)
      window.removeEventListener('mouseup', endDrag)
      window.removeEventListener('touchmove', onTouchMove)
      window.removeEventListener('touchend', endDrag)
      window.removeEventListener('touchcancel', endDrag)
    }
  }, [dragging, moveDrag, endDrag])

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

  if (!challenge) return null

  const imgW = challenge.width
  const imgH = challenge.height

  return (
    <Dialog open={open} onOpenChange={(o) => { if (!o) onCancel() }}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>安全验证</DialogTitle>
          <DialogDescription>
            {challenge.type === 'click' && `请依次点击：${challenge.text ?? ''}`}
            {challenge.type === 'slide' && '拖动滑块完成拼图'}
            {challenge.type === 'dragdrop' && '将拼图块拖到正确位置'}
            {challenge.type === 'rotate' && '拖动下方滑块旋转图片至正向'}
          </DialogDescription>
        </DialogHeader>
        <div ref={containerRef} className="relative select-none" style={{ width: imgW, maxWidth: '100%', aspectRatio: `${imgW} / ${imgH}` }}>
          <img src={challenge.image} alt="captcha" className="w-full rounded-md" onClick={onImageClick} />
          {(challenge.type === 'slide' || challenge.type === 'dragdrop') && challenge.thumb && (
            <img
              src={challenge.thumb}
              alt="puzzle piece"
              className="absolute cursor-grab"
              style={{
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
              value={angle}
              onChange={(e) => setAngle(normalizeAngle(Number(e.target.value)))}
              className="w-full"
              aria-label="旋转角度"
            />
          </div>
        )}
        <div className="flex gap-2">
          <Button variant="outline" className="flex-1" onClick={onRefresh}>换一题</Button>
          <Button className="flex-1" onClick={submit}>提交</Button>
        </div>
      </DialogContent>
    </Dialog>
  )
}
