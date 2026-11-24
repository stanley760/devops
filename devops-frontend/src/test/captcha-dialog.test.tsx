import { render, screen, fireEvent } from '@testing-library/react'
import { describe, it, expect, vi } from 'vitest'
import { CaptchaDialog } from '@/components/business/CaptchaDialog'
import { buildCaptchaAnswer } from '@/lib/captcha'
import type { CaptchaChallenge } from '@/lib/types'

const challenge = (over: Partial<CaptchaChallenge>): CaptchaChallenge => ({
  id: 'c1', type: 'slide', image: 'data:image/png;base64,x', width: 300, height: 150,
  thumb: 'data:image/png;base64,t', thumb_x: 10, thumb_y: 20, thumb_width: 60, thumb_height: 60,
  ...over,
})

function Harness({ challenge, onAnswer }: { challenge: CaptchaChallenge; onAnswer: (a: unknown) => void }) {
  return (
    <CaptchaDialog
      open
      challenge={challenge}
      onAnswer={(a) => onAnswer(a)}
      onRefresh={() => {}}
      onCancel={() => {}}
    />
  )
}

describe('CaptchaDialog interactions', () => {
  it('click type collects points and submits captcha_points', () => {
    const onAnswer = vi.fn()
    const c = challenge({ type: 'click', text: 'circle,star' })
    render(<Harness challenge={c} onAnswer={onAnswer} />)
    expect(screen.getByText(/circle,star/)).toBeInTheDocument()
    fireEvent.click(screen.getByAltText('captcha'))
    fireEvent.click(screen.getByText('提交'))
    expect(onAnswer).toHaveBeenCalledWith({
      points: [expect.objectContaining({ x: expect.any(Number), y: expect.any(Number) })],
    })
  })

  it('slide type submits captcha_x/captcha_y = base + offset via buildCaptchaAnswer', () => {
    const onAnswer = vi.fn()
    render(<Harness challenge={challenge({ type: 'slide' })} onAnswer={onAnswer} />)
    const piece = screen.getByAltText('puzzle piece')
    fireEvent.mouseDown(piece, { clientX: 10, clientY: 20 })
    fireEvent.mouseMove(piece, { clientX: 40, clientY: 20 })
    fireEvent.mouseUp(piece)
    fireEvent.click(screen.getByText('提交'))
    const a = onAnswer.mock.calls[0]![0] as { x: number; y: number }
    // 作答经 buildCaptchaAnswer 映射为后端契约字段
    const mapped = buildCaptchaAnswer(challenge({ type: 'slide' }), a)
    expect(mapped).toEqual({ captcha_id: 'c1', captcha_x: a.x, captcha_y: a.y })
  })

  it('rotate type defaults angle 0 when untouched', () => {
    const onAnswer = vi.fn()
    render(<Harness challenge={challenge({ type: 'rotate', thumb_size: 100 })} onAnswer={onAnswer} />)
    fireEvent.click(screen.getByText('提交'))
    const a = onAnswer.mock.calls[0]![0] as { angle: number }
    expect(buildCaptchaAnswer(challenge({ type: 'rotate' }), a)).toEqual({ captcha_id: 'c1', captcha_angle: 0 })
  })

  it('rotate drag accumulates across moves and submits a non-zero captcha_angle', () => {
    const onAnswer = vi.fn()
    const c = challenge({ type: 'rotate', width: 220, height: 220, thumb_size: 140 })
    render(<Harness challenge={c} onAnswer={onAnswer} />)
    const target = screen.getByAltText('rotate target')
    fireEvent.mouseDown(target, { clientX: 100, clientY: 100 })
    fireEvent.mouseMove(window, { clientX: 130, clientY: 100 })
    fireEvent.mouseMove(window, { clientX: 155, clientY: 100 })
    fireEvent.mouseUp(window)
    fireEvent.click(screen.getByText('提交'))
    const a = onAnswer.mock.calls[0]![0] as { angle: number }
    // 累计拖过 55px；图宽 220px 对应一整圈 → 55 * 360 / 220 = 90°。
    // 只取最后一帧增量会得到 25px → 40°，说明必须累计。
    expect(a.angle).toBe(90)
    expect(a.angle).not.toBe(0)
    expect(buildCaptchaAnswer(c, a)).toEqual({ captcha_id: 'c1', captcha_angle: 90 })
  })

  it('scales dragdrop coordinates from CSS display size back to image pixels', () => {
    const onAnswer = vi.fn()
    const c = challenge({ type: 'dragdrop', thumb_x: 10, thumb_y: 20, thumb_width: 60, thumb_height: 60 })
    render(<Harness challenge={c} onAnswer={onAnswer} />)
    // 图片自然宽 300px，容器被 CSS 缩放到 150px（缩放系数 2）。
    const container = screen.getByAltText('puzzle piece').parentElement!
    vi.spyOn(container, 'getBoundingClientRect').mockReturnValue({
      width: 150, height: 75, x: 0, y: 0, top: 0, left: 0, right: 150, bottom: 75, toJSON: () => ({}),
    } as DOMRect)
    const piece = screen.getByAltText('puzzle piece')
    fireEvent.mouseDown(piece, { clientX: 20, clientY: 20 })
    // 屏幕上移动 20px / 15px → 图片像素空间 40px / 30px。
    fireEvent.mouseMove(window, { clientX: 40, clientY: 35 })
    fireEvent.mouseUp(window)
    fireEvent.click(screen.getByText('提交'))
    const a = onAnswer.mock.calls[0]![0] as { x: number; y: number }
    expect(a.x).toBe(50) // thumb_x 10 + 20 * 2
    expect(a.y).toBe(50) // thumb_y 20 + 15 * 2
    expect(buildCaptchaAnswer(c, a)).toEqual({ captcha_id: 'c1', captcha_x: 50, captcha_y: 50 })
    vi.restoreAllMocks()
  })

  it('dragdrop type submits captcha_x/captcha_y', () => {
    const onAnswer = vi.fn()
    render(<Harness challenge={challenge({ type: 'dragdrop' })} onAnswer={onAnswer} />)
    const piece = screen.getByAltText('puzzle piece')
    fireEvent.mouseDown(piece, { clientX: 0, clientY: 0 })
    fireEvent.mouseMove(piece, { clientX: 15, clientY: 15 })
    fireEvent.mouseUp(piece)
    fireEvent.click(screen.getByText('提交'))
    const a = onAnswer.mock.calls[0]![0] as { x: number; y: number }
    expect(buildCaptchaAnswer(challenge({ type: 'dragdrop' }), a)).toMatchObject({ captcha_id: 'c1' })
  })
})
