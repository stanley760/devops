import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { applyAppearance } from '@/lib/appearance'
import PortalApp from './App'

applyAppearance()

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <PortalApp />
  </StrictMode>,
)
