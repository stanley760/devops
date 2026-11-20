import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { applyAppearance } from '@/lib/appearance'
import App from './App'

applyAppearance()

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
