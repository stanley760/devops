import '@testing-library/jest-dom/vitest'

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

beforeEach(() => {
  localStorage.clear()
})
