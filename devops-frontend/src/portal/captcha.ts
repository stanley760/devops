/**
 * 行为式验证码组件（点选 / 滑动 / 旋转），对接 devops-user 的 /auth/captcha。
 *
 * 与后端的约定：
 *   GET  /user/api/v1/auth/captcha → { id, type, image, thumb, width, height, text }
 *   提交：click     → captcha_points[{x,y}]（主图像素坐标）
 *         slide     → captcha_x / captcha_y
 *         dragdrop  → captcha_x / captcha_y
 *         rotate    → captcha_angle
 *
 * 题目要求的挑战是一次性的（服务端 GetDel），所以每次校验失败都必须重新拉取。
 */

export type CaptchaType = 'click' | 'slide' | 'dragdrop' | 'rotate'

export interface CaptchaChallenge {
  id: string
  type: CaptchaType
  image: string
  thumb?: string
  width: number
  height: number
  text?: string
}

export interface CaptchaAnswer {
  captcha_id: string
  captcha_points?: Array<{ x: number; y: number }>
  captcha_x?: number
  captcha_y?: number
  captcha_angle?: number
}

export class CaptchaWidget {
  readonly el: HTMLDivElement
  private challenge: CaptchaChallenge | null = null
  private points: Array<{ x: number; y: number }> = []
  private angle = 0
  private dragging = false
  private startX = 0
  private startY = 0
  private onSolved: (answer: CaptchaAnswer | null) => void
  private onRefresh: () => Promise<CaptchaChallenge>
  private statusEl: HTMLDivElement

  constructor(opts: {
    onSolved: (answer: CaptchaAnswer | null) => void
    onRefresh: () => Promise<CaptchaChallenge>
  }) {
    this.onSolved = opts.onSolved
    this.onRefresh = opts.onRefresh
    this.el = document.createElement('div')
    this.el.className = 'captcha'
    this.el.innerHTML = `
      <div class="captcha-stage" data-stage>
        <img class="captcha-img" data-img alt="captcha" />
        <div class="captcha-marks" data-marks></div>
        <div class="captcha-piece" data-piece hidden><img data-thumb alt="" /></div>
        <div class="captcha-rotate" data-rotate hidden><img data-rotate-img alt="" /></div>
        <div class="captcha-loading" data-loading>加载中…</div>
      </div>
      <div class="captcha-bar">
        <span class="captcha-tip" data-tip>获取验证码…</span>
        <button type="button" class="captcha-refresh" data-refresh>换一张</button>
      </div>`
    this.statusEl = this.el.querySelector('[data-tip]') as HTMLDivElement
  }

  /** 拉取新挑战并重置交互状态。 */
  async load(): Promise<void> {
    const loading = this.el.querySelector('[data-loading]') as HTMLElement
    loading.style.display = 'grid'
    try {
      const ch = await this.onRefresh()
      this.challenge = ch
      this.points = []
      this.angle = 0
      const img = this.el.querySelector('[data-img]') as HTMLImageElement
      const marks = this.el.querySelector('[data-marks]') as HTMLElement
      const piece = this.el.querySelector('[data-piece]') as HTMLElement
      const rot = this.el.querySelector('[data-rotate]') as HTMLElement
      const tip = this.el.querySelector('[data-tip]') as HTMLElement
      marks.innerHTML = ''
      img.src = ch.image
      img.hidden = false

      if (ch.type === 'click') {
        piece.hidden = true
        rot.hidden = true
        tip.textContent = ch.text ? `请点击：${ch.text}` : '请点击图中图形'
      } else if (ch.type === 'rotate') {
        piece.hidden = true
        rot.hidden = false
        const rimg = this.el.querySelector('[data-rotate-img]') as HTMLImageElement
        rimg.src = ch.thumb || ch.image
        tip.textContent = '拖动旋转图形至正确方向'
      } else {
        piece.hidden = false
        rot.hidden = true
        const timg = this.el.querySelector('[data-thumb]') as HTMLImageElement
        timg.src = ch.thumb || ''
        tip.textContent = ch.type === 'dragdrop' ? '拖动拼图到缺口处' : '拖动滑块完成拼图'
      }
    } catch (e) {
      this.statusEl.textContent = '验证码加载失败，点击换一张'
    } finally {
      loading.style.display = 'none'
    }
  }

  /** 收集当前交互结果；未完成时返回 null。 */
  answer(): CaptchaAnswer | null {
    const ch = this.challenge
    if (!ch) return null
    if (ch.type === 'click') {
      if (!this.points.length) return null
      return { captcha_id: ch.id, captcha_points: this.points }
    }
    if (ch.type === 'rotate') {
      return { captcha_id: ch.id, captcha_angle: this.angle }
    }
    return { captcha_id: ch.id, captcha_x: this.startX, captcha_y: this.startY }
  }

  /** 由外部（提交时）调用；返回是否已完成。 */
  isSolved(): boolean {
    return this.answer() !== null
  }

  bind(): void {
    const stage = this.el.querySelector('[data-stage]') as HTMLElement
    const marks = this.el.querySelector('[data-marks]') as HTMLElement
    const refresh = this.el.querySelector('[data-refresh]') as HTMLElement
    refresh.addEventListener('click', () => { void this.load() })

    stage.addEventListener('click', (e) => {
      if (this.challenge?.type !== 'click') return
      const rect = stage.getBoundingClientRect()
      const scaleX = this.challenge.width / rect.width
      const scaleY = this.challenge.height / rect.height
      const x = Math.round((e.clientX - rect.left) * scaleX)
      const y = Math.round((e.clientY - rect.top) * scaleY)
      this.points.push({ x, y })
      const dot = document.createElement('span')
      dot.className = 'captcha-dot'
      dot.style.left = `${((e.clientX - rect.left) / rect.width) * 100}%`
      dot.style.top = `${((e.clientY - rect.top) / rect.height) * 100}%`
      marks.appendChild(dot)
    })

    stage.addEventListener('pointerdown', (e) => {
      const ch = this.challenge
      if (!ch) return
      if (ch.type === 'click') return
      this.dragging = true
      this.startX = e.clientX - stage.getBoundingClientRect().left
      this.startY = e.clientY - stage.getBoundingClientRect().top
      stage.setPointerCapture(e.pointerId)
    })
    stage.addEventListener('pointermove', (e) => {
      if (!this.dragging || !this.challenge) return
      const rect = stage.getBoundingClientRect()
      const x = Math.min(Math.max(e.clientX - rect.left, 0), rect.width)
      const y = Math.min(Math.max(e.clientY - rect.top, 0), rect.height)
      if (this.challenge.type === 'rotate') {
        const cx = rect.width / 2
        const cy = rect.height / 2
        // 0° 指向正上方，顺时针为正
        const deg = Math.round((Math.atan2(x - cx, cy - y) * 180) / Math.PI)
        this.angle = (deg + 360) % 360
        const rimg = this.el.querySelector('[data-rotate-img]') as HTMLImageElement
        rimg.style.transform = `rotate(${this.angle}deg)`
      } else {
        this.startX = Math.round(x)
        this.startY = Math.round(y)
        const piece = this.el.querySelector('[data-piece]') as HTMLElement
        piece.style.left = `${x}px`
        piece.style.top = `${y}px`
      }
    })
    const end = (): void => {
      if (!this.dragging) return
      this.dragging = false
      if (this.challenge && this.challenge.type !== 'click') {
        this.onSolved(this.answer())
      }
    }
    stage.addEventListener('pointerup', end)
    stage.addEventListener('pointercancel', end)
  }
}
