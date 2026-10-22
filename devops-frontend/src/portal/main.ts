/**
 * C 端门户入口：hash 路由。
 *   #/          hero（Three.js 玻璃立方体首屏）
 *   #/login     登录
 *   #/register  注册
 *   #/profile   个人资料（需登录）
 *   #/docs      产品文档
 */
import './style.css'
import { mountHero, type HeroHandle } from './hero'
import { renderLogin, renderRegister, renderProfile, renderDocs } from './pages'

const app = document.getElementById('app')!

let hero: HeroHandle | null = null

function route(): void {
  hero?.destroy()
  hero = null
  const hash = location.hash || '#/'
  switch (hash) {
    case '#/login':
      renderLogin(app)
      break
    case '#/register':
      renderRegister(app)
      break
    case '#/profile':
      void renderProfile(app)
      break
    case '#/docs':
      renderDocs(app)
      break
    default:
      hero = mountHero(app)
  }
  window.scrollTo(0, 0)
}

window.addEventListener('hashchange', route)
route()
