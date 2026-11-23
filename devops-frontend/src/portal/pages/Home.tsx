interface HomeProps {
  navigate: (to: string) => void
  loggedIn: boolean
}

// 静态玻璃视觉营销首页（替代 Three.js hero，spec §1.3/§5.3），保留品牌文案
export function Home({ navigate, loggedIn }: HomeProps) {
  return (
    <div className="relative flex min-h-[70vh] flex-col items-center justify-center overflow-hidden">
      {/* 静态玻璃背景层（纯 CSS，无 WebGL） */}
      <div aria-hidden className="pointer-events-none absolute inset-0">
        <div className="bg-primary/10 absolute -left-24 top-10 h-72 w-72 rounded-full blur-3xl" />
        <div className="bg-primary/15 absolute right-0 bottom-0 h-96 w-96 rounded-full blur-3xl" />
      </div>
      <main className="glass-panel relative z-10 mx-4 max-w-2xl rounded-3xl p-10 text-center">
        <h1 className="text-5xl font-bold tracking-tight">Design World</h1>
        <p className="text-muted-foreground mt-4 text-xl">Explore New Ideas</p>
        <div className="mt-8 flex items-center justify-center gap-3">
          {!loggedIn && (
            <>
              <button
                className="bg-primary text-primary-foreground hover:bg-primary/90 rounded-full px-8 py-3 text-lg font-medium shadow-lg transition-transform hover:scale-105"
                onClick={() => navigate('/register')}
              >
                Explore Now
              </button>
              <button
                className="border-border hover:bg-accent rounded-full border px-8 py-3 text-lg transition-colors"
                onClick={() => navigate('/login')}
              >
                Sign In
              </button>
            </>
          )}
          {loggedIn && (
            <button
              className="bg-primary text-primary-foreground rounded-full px-8 py-3 text-lg font-medium"
              onClick={() => navigate('/profile')}
            >
              个人资料
            </button>
          )}
        </div>
        {!loggedIn && (
          <p className="text-muted-foreground mt-4 text-sm">
            还没有账号？
            <button type="button" className="text-primary underline-offset-4 hover:underline" onClick={() => navigate('/register')}>
              立即注册
            </button>
          </p>
        )}
      </main>
    </div>
  )
}
