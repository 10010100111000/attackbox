import type { Config } from 'tailwindcss'

export default {
  darkMode: ['class', '[data-theme="dark"]'],
  content: [
    './components/**/*.{vue,ts}',
    './layouts/**/*.vue',
    './pages/**/*.vue',
    './app.vue'
  ],
  theme: {
    extend: {
      colors: {
        background: 'var(--bg-app)',
        card: 'var(--bg-card)',
        input: 'var(--bg-input)',
        foreground: 'var(--text-main)',
        muted: 'var(--text-muted)',
        line: 'var(--border-line)',
        accent: {
          DEFAULT: 'var(--accent-coral)',
          hover: 'var(--accent-hover)',
          foreground: 'var(--text-on-accent)',
        },
        error: 'var(--color-error)'
      },
      borderRadius: {
        sm: 'var(--radius-sm)',
        md: 'var(--radius-md)',
        lg: 'var(--radius-lg)',
        xl: 'var(--radius-xl)'
      },
      fontFamily: {
        sans: ['-apple-system', 'BlinkMacSystemFont', '"Segoe UI"', '"PingFang SC"', '"Microsoft Yahei"', 'sans-serif'],
        mono: ['"SFMono-Regular"', 'Consolas', 'monospace']
      },
      backgroundImage: {
        'warm-gradient': 'var(--gradient-warm)'
      },
      boxShadow: {
        'card-glow': 'var(--card-glow)'
      }
    }
  }
} satisfies Config
