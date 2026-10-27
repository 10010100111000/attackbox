import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync, existsSync } from 'node:fs'
import { resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = fileURLToPath(new URL('.', import.meta.url))
const rootDir = resolve(__dirname, '..')

test('cn() utility merges classes and resolves tailwind conflicts', async () => {
  const { cn } = await import('../utils/cn.ts')
  assert.equal(typeof cn, 'function')
  assert.equal(cn('bg-red-500', 'bg-blue-500'), 'bg-blue-500')
  assert.equal(cn('p-4', { 'text-white': true, 'hidden': false }), 'p-4 text-white')
  assert.equal(cn('px-2 py-1', 'px-4'), 'py-1 px-4')
})

test('main.css contains all required design tokens and accessibility rules', () => {
  const css = readFileSync(resolve(rootDir, 'assets/css/main.css'), 'utf-8')
  
  // Theme tokens
  assert.ok(css.includes('--bg-app: #090a0e;'))
  assert.ok(css.includes('--bg-card: #111217;'))
  assert.ok(css.includes('--bg-input: #121318;'))
  assert.ok(css.includes('--text-main: #f2f0ed;'))
  assert.ok(css.includes('--text-muted: #96939d;'))
  assert.ok(css.includes('--border-line: rgba(255, 255, 255, 0.07);'))
  assert.ok(css.includes('--accent-coral: #ffaf88;'))
  assert.ok(css.includes('--accent-hover: #ffc19e;'))
  assert.ok(css.includes('--text-on-accent: #271a15;'))
  assert.ok(css.includes('--color-error: #e6a49c;'))
  assert.ok(css.includes('--gradient-warm:'))
  assert.ok(css.includes('--selection-bg:'))
  assert.ok(css.includes('--card-glow:'))
  
  // Radii
  assert.ok(css.includes('--radius-sm: 4px;'))
  assert.ok(css.includes('--radius-md: 7px;'))
  assert.ok(css.includes('--radius-lg: 11px;'))
  assert.ok(css.includes('--radius-xl: 18px;'))

  // Light theme with accessible error color
  assert.ok(css.includes("[data-theme='light']"))
  assert.ok(css.includes('--color-error: #c53030;'))
  
  // Accessibility
  assert.ok(css.includes(':focus-visible'))
  assert.ok(css.includes('prefers-reduced-motion: reduce'))
})

test('animations.css contains keyframes and animation utility classes', () => {
  const css = readFileSync(resolve(rootDir, 'assets/css/animations.css'), 'utf-8')
  assert.ok(css.includes('@keyframes blink'))
  assert.ok(css.includes('@keyframes spin'))
  assert.ok(css.includes('.animate-blink'))
  assert.ok(css.includes('.animate-spin-slow'))
})

test('tailwind.config.ts extends theme colors, radii tokens and card-glow shadow', async () => {
  const tailwindPath = resolve(rootDir, 'tailwind.config.ts')
  assert.ok(existsSync(tailwindPath))
  const content = readFileSync(tailwindPath, 'utf-8')
  assert.ok(content.includes('var(--bg-app)'))
  assert.ok(content.includes('var(--accent-coral)'))
  assert.ok(content.includes('var(--radius-sm)'))
  assert.ok(content.includes('var(--gradient-warm)'))
  assert.ok(content.includes("'card-glow': 'var(--card-glow)'"))
})

test('Button.vue defines required variants, sizes, loading indicator and link handling', () => {
  const content = readFileSync(resolve(rootDir, 'components/ui/Button.vue'), 'utf-8')
  assert.ok(content.includes('variant'))
  assert.ok(content.includes('primary'))
  assert.ok(content.includes('outline'))
  assert.ok(content.includes('ghost'))
  assert.ok(content.includes('size'))
  assert.ok(content.includes('loading'))
  assert.ok(content.includes('disabled'))
  assert.ok(content.includes('UiBusyLabel'))
  assert.ok(content.includes(':aria-busy="loading"'))
  assert.ok(content.includes('NuxtLink'))
  assert.ok(content.includes(':to="disabled || loading ? undefined : to"'))
  assert.ok(content.includes("props.loading ? 'cursor-wait'"))
})

test('Input.vue defines inheritAttrs: false, useId, blur event, password toggle, CapsLock detection and error handling', () => {
  const content = readFileSync(resolve(rootDir, 'components/ui/Input.vue'), 'utf-8')
  assert.ok(content.includes('inheritAttrs: false'))
  assert.ok(content.includes('useId'))
  assert.ok(content.includes("emit('blur'"))
  assert.ok(content.includes('v-bind="$attrs"'))
  assert.ok(content.includes('modelValue'))
  assert.ok(content.includes('password'))
  assert.ok(content.includes('Eye'))
  assert.ok(content.includes('EyeOff'))
  assert.ok(content.includes(':disabled="disabled"'))
  assert.ok(content.includes('CapsLock'))
  assert.ok(content.includes('大写锁定已开启'))
  assert.ok(content.includes('aria-invalid'))
  assert.ok(content.includes('error'))
})

test('Checkbox.vue integrates reka-ui and custom 13x13 coral checkbox styling', () => {
  const content = readFileSync(resolve(rootDir, 'components/ui/Checkbox.vue'), 'utf-8')
  assert.ok(content.includes('CheckboxRoot'))
  assert.ok(content.includes('CheckboxIndicator'))
  assert.ok(content.includes('h-[13px]'))
  assert.ok(content.includes('w-[13px]'))
  assert.ok(content.includes('data-[state=checked]:bg-accent'))
  assert.ok(content.includes('Check'))
})

test('Separator.vue supports horizontal line with centered text and left-right extensions with proper a11y', () => {
  const content = readFileSync(resolve(rootDir, 'components/ui/Separator.vue'), 'utf-8')
  assert.ok(content.includes('role="separator"'))
  assert.ok(content.includes('border-line'))
  assert.ok(content.includes('slots.default'))
  assert.ok(content.includes('aria-hidden="true"'))
})
