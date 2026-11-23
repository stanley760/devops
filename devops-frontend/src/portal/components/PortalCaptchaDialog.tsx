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

// portal 侧四题型验证码弹窗：click 点选 / slide+dragdrop 拖拽拼图 / rotate 旋转。
// 挑战一次性，校验失败由调用方重新拉题并复用本组件（onRefresh → 新 challenge.id 即重置交互）。
export function PortalCaptchaDialog({ open, challenge, onAnswer, onRefresh, onCancel }: Props) {
  const [points, setPoints] = useState<Array<{ x: number; y: number }>>([])
  const [offset, setOffset] = useState({ x: 0, y: 0 })
  const [angle, setAngle] = useState(0)
  const dragging = useRef(false)
  const dragStart = useRef({ x: 0, y: 0 })

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

  if (!challenge) return null

  const imgW = challenge.width
  const imgH = challenge.height

  const onImageClick = (e: React.MouseEvent<HTMLImageElement>) => {
    if (challenge.type !== 'click') return
    const rect = e.currentTarget.getBoundingClientRect()
    setPoints((p) => [
      ...p,
      {
        x: Math.round(((e.clientX - rect.left) * imgW) / rect.width),
        y: Math.round(((e.clientY - rect.top) * imgH) / rect.height),
      },
    ])
  }

  const onDragStart = (e: React.MouseEvent | React.TouchEvent) => {
    dragging.current = true
    const pt = 'touches' in e ? e.touches[0]! : e
    dragStart.current = { x: pt.clientX - offset.x, y: pt.clientY - offset.y }
  }
  const onDragMove = (e: React.MouseEvent | React.TouchEvent) => {
    if (!dragging.current) return
    const pt = 'touches' in e ? (e.touches[0] ?? e.changedTouches[0]) : e
    if (!pt) return
    if (challenge.type === 'rotate') {
      const dy = pt.clientY - dragStart.current.y
      dragStart.current = { x: pt.clientX, y: pt.clientY }
      setAngle(((offset.y + dy) % 360 + 360) % 360)
    } else {
      setOffset({
        x: Math.max(0, Math.min(pt.clientX - dragStart.current.x, imgW - (challenge.thumb_width ?? 0))),
        y: Math.max(0, pt.clientY - dragStart.current.y),
      })
    }
  }
  const onDragEnd = () => {
    dragging.current = false
  }

  return (
    <PortalDialog open={open} onOpenChange={(o) => { if (!o) onCancel() }} title="安全验证">
      <p className="text-muted-foreground mb-2 text-sm">
        {challenge.type === 'click' && `请依次点击：${challenge.text ?? ''}`}
        {challenge.type === 'slide' && '拖动滑块完成拼图'}
        {challenge.type === 'dragdrop' && '将拼图块拖到正确位置'}
        {challenge.type === 'rotate' && '拖动滑块旋转图片至正向'}
      </p>
      <div className="relative select-none" style={{ width: imgW, maxWidth: '100%', aspectRatio: `${imgW} / ${imgH}` }}>
        <img src={challenge.image} alt="captcha" className="w-full rounded-md" onClick={onImageClick} />
        {(challenge.type === 'slide' || challenge.type === 'dragdrop') && challenge.thumb && (
          <img
            src={challenge.thumb}
            alt="puzzle piece"
            className="absolute cursor-grab"
            style={{
              left: (challenge.thumb_x ?? 0) + offset.x,
              top: (challenge.thumb_y ?? 0) + offset.y,
              width: challenge.thumb_width,
              height: challenge.thumb_height,
            }}
            onMouseDown={onDragStart}
            onMouseMove={onDragMove}
            onMouseUp={onDragEnd}
            onMouseLeave={onDragEnd}
            onTouchStart={onDragStart}
            onTouchMove={onDragMove}
            onTouchEnd={onDragEnd}
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
            className="rounded-md"
          />
          <input
            type="range"
            min={0}
            max={360}
            value={angle}
            onChange={(e) => setAngle(Number(e.target.value))}
            className="w-full"
            aria-label="旋转角度"
          />
        </div>
      )}
      <div className="mt-3 flex gap-2">
        <Button variant="outline" className="flex-1" onClick={onRefresh}>换一题</Button>
        <Button className="flex-1" onClick={submit}>提交</Button>
      </div>
    </PortalDialog>
  )
}
