import '@testing-library/jest-dom/vitest'

// jsdom exposes fetch on globalThis; alias `global` for tests using vi.spyOn(global, 'fetch').
;(globalThis as typeof globalThis & { global: typeof globalThis }).global = globalThis

// jsdom does not implement matchMedia; provide a minimal spy-able implementation.
Object.defineProperty(window, 'matchMedia', {
  writable: true,
  value: (query: string) => ({
    media: query,
    matches: false,
    onchange: null,
    addEventListener: () => {},
    removeEventListener: () => {},
  }),
})

// jsdom does not implement ResizeObserver (needed by cmdk).
if (!('ResizeObserver' in globalThis)) {
  Object.defineProperty(globalThis, 'ResizeObserver', {
    writable: true,
    value: class {
      observe() {}
      unobserve() {}
      disconnect() {}
    },
  })
}

// jsdom does not implement Element.scrollIntoView (needed by cmdk).
if (!Element.prototype.scrollIntoView) {
  Element.prototype.scrollIntoView = () => {}
}

beforeEach(() => {
  localStorage.clear()
})
