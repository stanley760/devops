/// <reference types="vite/client" />

// Vitest tests spy on Node's `global`; jsdom's DOM lib does not declare it.
declare const global: typeof globalThis
