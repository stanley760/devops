/**
 * C 端门户 hero：黑色 "Design World" 首屏。
 * 中央可旋转玻璃立方体，屏幕空间折射背景大标题（6 带色散）。
 * 渲染管线：bg → rtBack；bg+背面 → rtFront；bg+正面 → 屏幕。
 */
import * as THREE from 'three'
import { GLTFLoader } from 'three/addons/loaders/GLTFLoader.js'
import { mergeVertices, mergeGeometries } from 'three/addons/utils/BufferGeometryUtils.js'
import { RoundedBoxGeometry } from 'three/addons/geometries/RoundedBoxGeometry.js'

const MODEL_URL = 'https://motionsites.org/assets/prompt-media/fa96096e78c8153a70d2.glb'

const HEADLINE_LINES = ['Explore', 'New', 'Ideas']
const HEADLINE_COLOR = '#e9e9e9'

export interface HeroHandle {
  destroy(): void
}

export function mountHero(root: HTMLElement): HeroHandle {
  root.innerHTML = `
  <section class="hero" id="hero">
    <canvas id="scene" aria-label="Rotatable glass cube. Drag to rotate."></canvas>
    <div class="ui">
      <h1 class="sr-only">Explore New Ideas</h1>
      <header class="nav">
        <a href="#/" class="logo"><i class="logo-mark"></i><b>Design</b><span>World</span></a>
        <ul class="nav-links">
          <li><a href="#/">Home</a></li>
          <li class="hide-sm"><a href="#/docs">产品文档</a></li>
          <li><a href="#/login">登录</a></li>
          <li><a href="#/register" class="solid">注册</a></li>
        </ul>
      </header>
      <div class="arrows">
        <button class="arrow" id="prev" aria-label="Rotate left">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.6" stroke-linecap="round" stroke-linejoin="round"><path d="M19 12H5M11 6l-6 6 6 6"/></svg>
        </button>
        <button class="arrow" id="next" aria-label="Rotate right">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.6" stroke-linecap="round" stroke-linejoin="round"><path d="M5 12h14M13 6l6 6-6 6"/></svg>
        </button>
      </div>
      <nav class="dots">
        <button class="dot" aria-label="Slide 1"></button>
        <button class="dot active" aria-label="Slide 2"></button>
        <button class="dot" aria-label="Slide 3"></button>
      </nav>
      <p class="tagline">Let's Build the<br>Future of <strong>Design.</strong></p>
      <div class="cta-row">
        <a href="#/register" class="cta">Explore Now</a>
        <span class="cta-line"></span>
        <span class="count" aria-hidden="true">07</span>
      </div>
      <div class="loader" id="loader">Loading model</div>
    </div>
  </section>`

  const canvas = root.querySelector<HTMLCanvasElement>('#scene')!
  const loaderEl = root.querySelector<HTMLElement>('#loader')!
  const heroEl = root.querySelector<HTMLElement>('#hero')!

  // ---------- renderer / camera ----------
  const renderer = new THREE.WebGLRenderer({ canvas, antialias: true, alpha: false })
  renderer.setClearColor(0x000000, 1)
  renderer.outputColorSpace = THREE.SRGBColorSpace
  const dpr = Math.min(window.devicePixelRatio || 1, 2)
  renderer.setPixelRatio(dpr)

  const camera = new THREE.PerspectiveCamera(30, 1, 0.1, 100)
  camera.position.set(0, 0, 10)
  camera.lookAt(0, 0, 0)

  // ---------- background headline (2D canvas → CanvasTexture) ----------
  const bgCanvas = document.createElement('canvas')
  const bgCtx = bgCanvas.getContext('2d')!
  const bgTex = new THREE.CanvasTexture(bgCanvas)
  bgTex.colorSpace = THREE.SRGBColorSpace
  bgTex.minFilter = THREE.LinearFilter
  bgTex.magFilter = THREE.LinearFilter
  bgTex.generateMipmaps = false

  let W = 2, H = 2, mobile = false

  function drawHeadline(): void {
    bgCanvas.width = Math.round(W * dpr)
    bgCanvas.height = Math.round(H * dpr)
    bgCtx.setTransform(dpr, 0, 0, dpr, 0, 0)
    bgCtx.fillStyle = '#000'
    bgCtx.fillRect(0, 0, W, H)

    let fs = Math.min(H * 0.21, W * (mobile ? 0.21 : 0.118))
    const setFont = () => { bgCtx.font = `800 ${fs}px Poppins, sans-serif` }
    setFont()
    const maxW = Math.max(...HEADLINE_LINES.map((t) => bgCtx.measureText(t).width))
    const limit = W * (mobile ? 0.9 : 0.5)
    if (maxW > limit) {
      fs *= limit / maxW
      setFont()
    }

    const cx = W * (mobile ? 0.5 : 0.505)
    const cy = H * (mobile ? 0.45 : 0.468)
    const gap = fs * 1.07
    const cap = fs * 0.7
    bgCtx.fillStyle = HEADLINE_COLOR
    bgCtx.textAlign = 'center'
    bgCtx.textBaseline = 'alphabetic'
    HEADLINE_LINES.forEach((t, i) => {
      bgCtx.fillText(t, cx, cy + cap / 2 + (i - 1) * gap)
    })
    bgTex.needsUpdate = true
  }

  // fullscreen quad in its own scene
  const bgScene = new THREE.Scene()
  const bgMat = new THREE.ShaderMaterial({
    uniforms: { uTex: { value: bgTex } },
    vertexShader: /* glsl */ `
      varying vec2 vUv;
      void main() {
        vUv = uv;
        gl_Position = vec4(position.xy, 0.0, 1.0);
      }`,
    fragmentShader: /* glsl */ `
      uniform sampler2D uTex;
      varying vec2 vUv;
      void main() {
        gl_FragColor = texture2D(uTex, vUv);
        #include <colorspace_fragment>
      }`,
    depthTest: false,
    depthWrite: false,
  })
  const bgQuad = new THREE.Mesh(new THREE.PlaneGeometry(2, 2), bgMat)
  bgQuad.frustumCulled = false
  bgScene.add(bgQuad)

  // ---------- glass cube ----------
  const scene = new THREE.Scene()
  const pivot = new THREE.Group()
  const spinner = new THREE.Group()
  spinner.rotation.set(-0.42, 0.62, 0.18)
  pivot.add(spinner)
  scene.add(pivot)

  const glassVertex = /* glsl */ `
    varying vec3 vNormal;
    varying vec3 vEye;
    void main() {
      vec4 worldPos = modelMatrix * vec4(position, 1.0);
      vec4 mvPos = viewMatrix * worldPos;
      gl_Position = projectionMatrix * mvPos;
      vNormal = normalize(normalMatrix * normal);
      vEye = normalize(mvPos.xyz);
    }`

  const glassFragment = /* glsl */ `
    uniform sampler2D uTexture;
    uniform vec2 uResolution;
    uniform float uIorR, uIorY, uIorG, uIorC, uIorB, uIorP;
    uniform float uRefractPower, uChromatic, uSaturation;
    uniform float uShininess, uDiffuseness, uFresnelPower, uBackside;
    uniform vec3 uLight;
    varying vec3 vNormal;
    varying vec3 vEye;

    float specular(vec3 light, float shininess, float diffuseness, vec3 n, vec3 eye) {
      vec3 lightVec = normalize(-light);
      vec3 viewVec = -eye;
      vec3 halfVec = normalize(viewVec + lightVec);
      return pow(max(dot(n, halfVec), 0.0), shininess) + max(0.0, dot(n, lightVec)) * diffuseness;
    }

    void main() {
      vec2 uv = gl_FragCoord.xy / uResolution;
      vec3 n = normalize(vNormal);
      if (uBackside > 0.5) n = -n;
      vec3 eye = normalize(vEye);

      const int LOOP = 16;
      vec3 color = vec3(0.0);
      for (int i = 0; i < LOOP; i++) {
        float slide = float(i) / float(LOOP) * 0.045;

        vec3 refrR = refract(eye, n, 1.0 / uIorR);
        float r = texture2D(uTexture, uv + refrR.xy * (uRefractPower + slide * 1.0) * uChromatic).x * 0.5;

        vec3 refrY = refract(eye, n, 1.0 / uIorY);
        vec3 ty = texture2D(uTexture, uv + refrY.xy * (uRefractPower + slide * 1.0) * uChromatic).xyz;
        float y = (ty.x * 2.0 + ty.y * 2.0 - ty.z) / 6.0;

        vec3 refrG = refract(eye, n, 1.0 / uIorG);
        float g = texture2D(uTexture, uv + refrG.xy * (uRefractPower + slide * 2.0) * uChromatic).y * 0.5;

        vec3 refrC = refract(eye, n, 1.0 / uIorC);
        vec3 tc = texture2D(uTexture, uv + refrC.xy * (uRefractPower + slide * 2.5) * uChromatic).xyz;
        float c = (tc.y * 2.0 + tc.z * 2.0 - tc.x) / 6.0;

        vec3 refrB = refract(eye, n, 1.0 / uIorB);
        float b = texture2D(uTexture, uv + refrB.xy * (uRefractPower + slide * 3.0) * uChromatic).z * 0.5;

        vec3 refrP = refract(eye, n, 1.0 / uIorP);
        vec3 tp = texture2D(uTexture, uv + refrP.xy * (uRefractPower + slide * 1.0) * uChromatic).xyz;
        float p = (tp.z * 2.0 + tp.x * 2.0 - tp.y) / 6.0;

        float R = r + (2.0 * p + 2.0 * y - c) / 3.0;
        float G = g + (2.0 * y + 2.0 * c - p) / 3.0;
        float B = b + (2.0 * c + 2.0 * p - y) / 3.0;
        color += vec3(R, G, B);
      }
      color /= float(LOOP);

      float luma = dot(color, vec3(0.2125, 0.7154, 0.0721));
      color = mix(vec3(luma), color, uSaturation);

      float spec = specular(uLight, uShininess, uDiffuseness, n, eye)
        + 0.6 * specular(vec3(1.0, 1.0, -1.0), uShininess * 0.6, uDiffuseness * 0.5, n, eye);
      color += spec * (uBackside > 0.5 ? 0.35 : 1.0);

      float f = pow(1.0 + dot(eye, n), uFresnelPower);
      color = mix(color, vec3(1.0), f * (uBackside > 0.5 ? 0.25 : 0.55));

      color += vec3(0.004, 0.005, 0.007);
      gl_FragColor = vec4(color, 1.0);
      #include <colorspace_fragment>
    }`

  function makeGlassMaterial(side: THREE.Side, backside: boolean): THREE.ShaderMaterial {
    return new THREE.ShaderMaterial({
      vertexShader: glassVertex,
      fragmentShader: glassFragment,
      side,
      uniforms: {
        uTexture: { value: null },
        uResolution: { value: new THREE.Vector2(1, 1) },
        uIorR: { value: 1.15 }, uIorY: { value: 1.16 }, uIorG: { value: 1.18 },
        uIorC: { value: 1.22 }, uIorB: { value: 1.22 }, uIorP: { value: 1.22 },
        uRefractPower: { value: backside ? 0.22 : 0.30 },
        uChromatic: { value: 0.5 },
        uSaturation: { value: 1.08 },
        uShininess: { value: 90 },
        uDiffuseness: { value: 0.02 },
        uFresnelPower: { value: 5.0 },
        uLight: { value: new THREE.Vector3(-1, 1, 1) },
        uBackside: { value: backside ? 1 : 0 },
      },
    })
  }

  const backMat = makeGlassMaterial(THREE.BackSide, true)
  const frontMat = makeGlassMaterial(THREE.FrontSide, false)

  const rtOpts: THREE.RenderTargetOptions = { type: THREE.HalfFloatType }
  let rtBack = new THREE.WebGLRenderTarget(2, 2, rtOpts)
  let rtFront = new THREE.WebGLRenderTarget(2, 2, rtOpts)

  let cube: THREE.Mesh | null = null

  function buildFallback(): void {
    attachGeometry(new RoundedBoxGeometry(1, 1, 1, 8, 0.12))
  }

  function attachGeometry(geometry: THREE.BufferGeometry): void {
    geometry.computeBoundingBox()
    const bb = geometry.boundingBox!
    const centre = new THREE.Vector3()
    bb.getCenter(centre)
    geometry.translate(-centre.x, -centre.y, -centre.z)
    const size = new THREE.Vector3()
    bb.getSize(size)
    const s = 1 / Math.max(size.x, size.y, size.z)
    geometry.scale(s, s, s)
    cube = new THREE.Mesh(geometry, frontMat)
    spinner.add(cube)
    loaderEl.classList.add('done')
    layout()
  }

  new GLTFLoader().load(
    MODEL_URL,
    (gltf) => {
      const parts: THREE.BufferGeometry[] = []
      gltf.scene.updateMatrixWorld(true)
      gltf.scene.traverse((obj) => {
        const mesh = obj as THREE.Mesh
        if (!mesh.isMesh) return
        let g = mesh.geometry.clone() as THREE.BufferGeometry
        g.deleteAttribute('uv')
        g.deleteAttribute('color')
        g.deleteAttribute('tangent')
        g = mergeVertices(g, 1e-4)
        g.computeVertexNormals()
        g.applyMatrix4(mesh.matrixWorld)
        parts.push(g)
      })
      if (!parts.length) { buildFallback(); return }
      attachGeometry(parts.length > 1 ? mergeGeometries(parts)! : parts[0])
    },
    undefined,
    () => buildFallback(),
  )

  // ---------- layout ----------
  const uResFront = frontMat.uniforms.uResolution.value as THREE.Vector2
  const uResBack = backMat.uniforms.uResolution.value as THREE.Vector2

  function layout(): void {
    W = heroEl.clientWidth || window.innerWidth
    H = heroEl.clientHeight || window.innerHeight
    mobile = W < 768 || W / H < 1

    renderer.setSize(W, H, false)
    camera.aspect = W / H
    camera.updateProjectionMatrix()

    const dbW = Math.round(W * dpr)
    const dbH = Math.round(H * dpr)
    rtBack.setSize(dbW, dbH)
    rtFront.setSize(dbW, dbH)
    uResFront.set(dbW, dbH)
    uResBack.set(dbW, dbH)

    drawHeadline()

    const visH = 2 * Math.tan(THREE.MathUtils.degToRad(camera.fov / 2)) * camera.position.z
    const visW = visH * camera.aspect
    const sx = mobile ? 0.5 : 0.517
    const sy = mobile ? 0.45 : 0.488
    pivot.position.set((sx - 0.5) * visW, (0.5 - sy) * visH, 0)
    const px = Math.min(H * 0.44, W * (mobile ? 0.45 : 0.29))
    pivot.scale.setScalar((px / H) * visH)
  }

  // ---------- interaction ----------
  let dragging = false
  let lastX = 0, lastY = 0
  let velX = 0, velY = 0 // rad per 16.67ms frame
  let lastMoveT = 0
  let spinRemaining = 0 // arrow spin
  let idleBlend = 0
  let idleDelay = 0 // seconds since release
  let rafId = 0
  let destroyed = false
  let prevT = performance.now()

  const tmpQ = new THREE.Quaternion()
  const AXIS_Y = new THREE.Vector3(0, 1, 0)
  const AXIS_X = new THREE.Vector3(1, 0, 0)

  function rotateWorld(axis: THREE.Vector3, angle: number): void {
    tmpQ.setFromAxisAngle(axis, angle)
    spinner.quaternion.premultiply(tmpQ)
  }

  function onPointerDown(e: PointerEvent): void {
    dragging = true
    spinRemaining = 0
    velX = velY = 0
    lastX = e.clientX
    lastY = e.clientY
    lastMoveT = performance.now()
    idleBlend = 0
    idleDelay = 0
    canvas.classList.add('dragging')
    canvas.setPointerCapture(e.pointerId)
  }
  function onPointerMove(e: PointerEvent): void {
    if (!dragging) return
    const dx = (e.clientX - lastX) * 0.008
    const dy = (e.clientY - lastY) * 0.008
    lastX = e.clientX
    lastY = e.clientY
    rotateWorld(AXIS_Y, dx)
    rotateWorld(AXIS_X, dy)
    const now = performance.now()
    const frames = Math.max((now - lastMoveT) / 16.67, 0.5)
    velX = dx / frames
    velY = dy / frames
    lastMoveT = now
  }
  function onPointerUp(e: PointerEvent): void {
    if (!dragging) return
    dragging = false
    idleDelay = 0
    canvas.classList.remove('dragging')
    try { canvas.releasePointerCapture(e.pointerId) } catch { /* noop */ }
  }
  canvas.addEventListener('pointerdown', onPointerDown)
  canvas.addEventListener('pointermove', onPointerMove)
  canvas.addEventListener('pointerup', onPointerUp)
  canvas.addEventListener('pointercancel', onPointerUp)

  let spinDir = 1
  root.querySelector('#prev')!.addEventListener('click', () => {
    spinDir = -1
    spinRemaining += Math.PI / 2
    velX = velY = 0
    idleBlend = 0
  })
  root.querySelector('#next')!.addEventListener('click', () => {
    spinDir = 1
    spinRemaining += Math.PI / 2
    velX = velY = 0
    idleBlend = 0
  })

  root.querySelectorAll('.dot').forEach((d) => {
    d.addEventListener('click', () => {
      root.querySelectorAll('.dot').forEach((o) => o.classList.remove('active'))
      d.classList.add('active')
    })
  })

  // ---------- render loop ----------
  function frame(now: number): void {
    if (destroyed) return
    rafId = requestAnimationFrame(frame)
    const dt = Math.min((now - prevT) / 1000, 0.05)
    prevT = now

    if (!dragging && cube) {
      if (spinRemaining > 0.0005) {
        const step = spinRemaining * Math.min(1, 0.09 * dt * 60)
        rotateWorld(AXIS_Y, step * spinDir)
        spinRemaining -= step
      } else {
        spinRemaining = 0
        // inertia
        const damp = Math.pow(0.94, dt * 60)
        velX *= damp
        velY *= damp
        if (Math.abs(velX) > 1e-5 || Math.abs(velY) > 1e-5) {
          rotateWorld(AXIS_Y, velX * dt * 60)
          rotateWorld(AXIS_X, velY * dt * 60)
        }
        // idle drift：0.6s 后 1s 内渐入
        idleDelay += dt
        const target = idleDelay > 0.6 ? 1 : 0
        idleBlend += (target - idleBlend) * Math.min(1, dt)
        if (idleBlend > 0.001) {
          rotateWorld(AXIS_Y, 0.0035 * idleBlend * dt * 60)
          rotateWorld(AXIS_X, 0.0012 * idleBlend * dt * 60)
        }
      }
    }

    if (cube) {
      backMat.uniforms.uTexture.value = rtBack.texture
      frontMat.uniforms.uTexture.value = rtFront.texture

      // 1) bg → rtBack
      renderer.setRenderTarget(rtBack)
      renderer.render(bgScene, camera)

      // 2) bg + back faces → rtFront
      renderer.setRenderTarget(rtFront)
      renderer.render(bgScene, camera)
      cube.material = backMat
      renderer.autoClear = false
      renderer.render(scene, camera)

      // 3) bg + front faces → screen
      renderer.setRenderTarget(null)
      renderer.autoClear = true
      renderer.render(bgScene, camera)
      cube.material = frontMat
      renderer.autoClear = false
      renderer.clearDepth()
      renderer.render(scene, camera)
      renderer.autoClear = true
    } else {
      renderer.setRenderTarget(null)
      renderer.render(bgScene, camera)
    }
  }

  function onResize(): void { layout() }
  window.addEventListener('resize', onResize)

  // 等字体就绪后再绘制大标题并启动循环
  const fontsReady = Promise.race([
    Promise.all([document.fonts.load('800 100px Poppins'), document.fonts.ready]),
    new Promise((r) => setTimeout(r, 2500)),
  ])
  void fontsReady.then(() => {
    if (destroyed) return
    layout()
    prevT = performance.now()
    rafId = requestAnimationFrame(frame)
  })
  const onFontsDone = (): void => { if (!destroyed) drawHeadline() }
  document.fonts.addEventListener('loadingdone', onFontsDone)

  return {
    destroy() {
      destroyed = true
      cancelAnimationFrame(rafId)
      window.removeEventListener('resize', onResize)
      document.fonts.removeEventListener('loadingdone', onFontsDone)
      rtBack.dispose()
      rtFront.dispose()
      bgTex.dispose()
      bgMat.dispose()
      backMat.dispose()
      frontMat.dispose()
      cube?.geometry.dispose()
      renderer.dispose()
    },
  }
}
