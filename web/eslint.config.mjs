// @ts-check
import withNuxt from './.nuxt/eslint.config.mjs'

export default withNuxt({
  rules: {
    // 页面与布局使用 Nuxt 的文件路由命名,不适用多词组件名规则
    'vue/multi-word-component-names': 'off',
    // 逐层收敛的历史欠账:先保留告警,不阻塞提交
    '@typescript-eslint/no-explicit-any': 'warn',
    // Markdown 渲染结果由 utils/markdown.ts 统一转义,组件内不再重复限制
    'vue/no-v-html': 'off'
  }
})
