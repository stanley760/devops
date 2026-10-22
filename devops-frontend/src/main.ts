import { createApp } from 'vue'
import App from './App.vue'
import { router } from './router'

// 样式引入顺序即层叠顺序：令牌 → 基础 → 材质 → 控件 → 仪表 → 布局 → 页面
import './styles/tokens.css'
import './styles/base.css'
import './styles/materials.css'
import './styles/components.css'
import './styles/instruments.css'
import './styles/layout.css'
import './styles/pages.css'

// 外观必须在挂载前写入 data-theme / data-density，避免主题闪白
import './composables/appearance'
import { installHoverLift, installPressFeedback, installRipple } from './composables/motion'

const app = createApp(App)
app.use(router)
app.mount('#app')

// 全局委托微交互只装一次
installPressFeedback()
installHoverLift()
installRipple()
