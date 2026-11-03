import { createRequire } from 'node:module'
import type { Config } from 'tailwindcss'

const require = createRequire(import.meta.url)
const plugins: NonNullable<Config['plugins']> = []
try {
  plugins.push(require('@tailwindcss/typography'))
} catch {
  /* Falls back to prose typography rules in assets/css/markdown.css if not yet installed */
}

export default {
  darkMode: ['class', '[data-theme="midnight"]'],
  content: ['./components/**/*.{vue,ts}', './layouts/**/*.vue', './pages/**/*.vue', './app.vue'],
  theme: {
    extend: {
      colors: {
        background: 'var(--bg, var(--bg-app))',
        card: 'var(--paper, var(--bg-card))',
        input: 'var(--bg-input)',
        foreground: 'var(--ink, var(--text-main))',
        muted: 'var(--muted, var(--text-muted))',
        line: 'var(--line, var(--border-line))',
        paper: 'var(--paper)',
        ink: 'var(--ink)',
        brand: 'var(--brand)',
        accent: {
          DEFAULT: 'var(--accent-coral)',
          hover: 'var(--accent-hover)',
          foreground: 'var(--text-on-accent)'
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
        sans: [
          'Inter',
          '-apple-system',
          'BlinkMacSystemFont',
          '"HarmonyOS Sans SC"',
          '"MiSans"',
          '"PingFang SC"',
          '"Hiragino Sans GB"',
          '"Noto Sans SC"',
          '"Microsoft YaHei UI"',
          '"Microsoft Yahei"',
          '"Segoe UI"',
          'sans-serif'
        ],
        mono: [
          '"JetBrains Mono"',
          'ui-monospace',
          '"SFMono-Regular"',
          '"Cascadia Code"',
          'Consolas',
          'monospace'
        ]
      },
      backgroundImage: {
        'warm-gradient': 'var(--gradient-warm)'
      },
      boxShadow: {
        'card-glow': 'var(--card-glow)'
      }
    }
  },
  plugins
} satisfies Config
