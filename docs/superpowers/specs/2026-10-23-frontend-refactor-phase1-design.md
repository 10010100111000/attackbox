# AttackBox 前端重构设计规范 (Phase 1)

**日期**：2026-10-23  
**状态**：已评审 (Pending User Review)  
**阶段**：第一阶段 (Phase 1) — Nuxt 项目搭建、设计系统规范与核心页面（首页、认证页）迁入

---

## 1. 目标与背景

AttackBox 原有前端包含分散的静态原型设计与旧版控制台界面。为了统一架构并逐步替换冗余的 Control API，前端技术栈全面切换为 **Nuxt 3 + TypeScript + Tailwind CSS + Reka UI**。

整体重构按 4 个阶段演化：
1. **Phase 1（当前）**：建立独立 `web/` Nuxt 工程与设计基础，迁入已确认的原型首页（landing-page）与登录/注册页（auth），统一字体、颜色、间距与通用组件。
2. **Phase 2**：接入 PostgreSQL + Drizzle ORM，跑通管理员创建模块、编辑章节、发布内容与真实认证。
3. **Phase 3**：完成学习闭环（目录、正文、答案校验、解锁下一章、保存进度）。
4. **Phase 4**：接入沙箱 CLI/GUI（xterm.js / noVNC），逐步替代并下线 Go Control 中的冗余网站接口。

本规范严格聚焦于 **Phase 1** 的交付范围。

---

## 2. 技术选型与依赖配置

| 职责分工 | 技术选型 | 版本/说明 |
| :--- | :--- | :--- |
| 全栈框架 | Nuxt 3 (Node.js 运行环境) | `^3.15.x`，混合渲染支持（SSR / 预渲染） |
| 编程语言 | TypeScript | 严格模式 (`strict: true`) |
| 样式系统 | Tailwind CSS + CSS 变量 | `@nuxtjs/tailwindcss ^6.13.x` |
| 基础无样式交互 | Reka UI | `reka-ui ^2.x`（及其 `reka-ui/nuxt` 模块） |
| 图标库 | Lucide Vue Next | `lucide-vue-next` |

---

## 3. 目录与工程结构

在工程根目录下新建 `web/` 目录，与现有 Go 服务及旧 `frontend/` 物理隔离：

```text
attackbox/
├── web/                           # Nuxt 3 独立前端工程
│   ├── assets/
│   │   ├── css/
│   │   │   ├── main.css           # 全局样式、CSS 变量与基础重置
│   │   │   └── animations.css     # 终端光标闪烁、微光扫描与特定过渡
│   │   └── images/
│   │       ├── linux.jpg          # Linux 模块底图
│   │       ├── network.jpg        # 网络模块底图
│   │       └── web.jpg            # Web 模块底图
│   ├── components/
│   │   ├── ui/                    # 基于 Reka UI 的原子通用组件
│   │   │   ├── Button.vue         # 按钮（Primary / Outline / Ghost / Loading）
│   │   │   ├── Input.vue          # 输入框（带错误提示、密码切换、大写锁定）
│   │   │   ├── Checkbox.vue       # 状态保持复选框（Reka UI CheckboxRoot）
│   │   │   └── Separator.vue      # 分割线（Reka UI Separator）
│   │   ├── landing/               # 落地页业务组件
│   │   │   ├── TerminalBackdrop.vue # 模糊终端背景自动演示
│   │   │   ├── HeroSection.vue    # 首屏主标题与行动按钮
│   │   │   ├── FeaturedModule.vue # 云原生攻防大幅主推卡片
│   │   │   └── ModuleRail.vue     # 横向无缝缓慢循环滚动模块架
│   │   └── auth/                  # 认证业务组件
│   │       ├── AuthVisualAside.vue# 桌面左侧品牌与命令演进装饰
│   │       ├── LoginForm.vue      # 邮箱/密码登录表单
│   │       ├── RegisterForm.vue   # 注册表单
│   │       └── RecoverForm.vue    # 密码找回表单
│   ├── composables/
│   │   ├── useTerminalPlayer.ts   # 终端剧本自动演练状态机 Composable
│   │   └── useModuleRail.ts       # 模块架无缝循环滚动机 Composable
│   ├── layouts/
│   │   ├── landing.vue            # 落地页精简布局（品牌头 + 页脚）
│   │   └── auth.vue               # 认证页全屏响应式布局（左右分栏 / 手机端居中）
│   ├── pages/
│   │   ├── index.vue              # 品牌落地页（预渲染）
│   │   └── auth.vue               # 统一认证入口（SSR + 客户端交互）
│   ├── public/
│   │   └── favicon.svg            # 珊瑚色 AttackBox 图标
│   ├── app.vue                    # 全局挂载与路由视图
│   ├── nuxt.config.ts             # Nuxt 模块与路由渲染规则配置
│   ├── package.json
│   ├── tailwind.config.ts         # Tailwind 与 CSS 变量桥接配置
│   └── tsconfig.json
├── prototypes/                    # 原始设计原型（保留作参考）
├── frontend/                      # 旧版前端（保留作对照）
├── control/                       # Go 现有后端服务
└── docs/
```

---

## 4. 样式系统与设计 Token 规范

### 4.1 全局 CSS 变量 (`assets/css/main.css`)

统一落地页与认证页的深色设计风格，并预留亮色（石墨）语义映射：

```css
:root {
  color-scheme: dark;

  /* 基础背景与文字色（深海暗色默认） */
  --bg-app: #090a0e;
  --bg-card: #111217;
  --bg-input: #121318;
  --text-main: #f2f0ed;
  --text-muted: #96939d;
  --border-line: rgba(255, 255, 255, 0.07);

  /* 品牌高亮色与交互态 */
  --accent-coral: #ffaf88;
  --accent-hover: #ffc19e;
  --text-on-accent: #271a15;
  --color-error: #e6a49c;

  /* 渐变与阴影 Token */
  --gradient-warm: linear-gradient(100deg, #ffe0b0, #ffae82 48%, #ef7c8d);
  --selection-bg: rgba(255, 175, 136, 0.27);
  --card-glow: 0 15px 45px rgba(0, 0, 0, 0.4);

  /* 统一圆角 */
  --radius-sm: 4px;
  --radius-md: 7px;
  --radius-lg: 11px;
  --radius-xl: 18px;
}

/* 石墨亮色模式（预留供 Phase 2/3 学习工作台使用） */
[data-theme='light'] {
  color-scheme: light;
  --bg-app: #f9f8f6;
  --bg-card: #ffffff;
  --bg-input: #f2f0ed;
  --text-main: #1d1b20;
  --text-muted: #736d7a;
  --border-line: rgba(0, 0, 0, 0.08);
  --text-on-accent: #271a15;
}
```

### 4.2 Tailwind 配置 (`tailwind.config.ts`)

```typescript
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
      }
    }
  }
} satisfies Config
```

---

## 5. 组件规范与动效 Composable

### 5.1 通用原子组件 (`components/ui/`)

1. **`Button.vue`**
   - 变体属性 `variant`: `'primary' | 'outline' | 'ghost'`
   - 加载状态 `loading`: 禁用点击并渲染平滑加载小圆点（`loading-dot`）。
   - 支持多态标签：通过 `as` 属性兼容 `NuxtLink` 或原生 `button`。
2. **`Input.vue`**
   - 类型 `type`: `'text' | 'email' | 'password'`
   - 错误状态: 通过 `aria-invalid` 动态关联 `field-error` 提示。
   - 密码专用特性: 当 `type="password"` 时，内嵌眼睛图标用于切换明文/暗文；监听 `keydown/keyup` 实时检测 `CapsLock` 并弹出“大写锁定已开启”提示。
3. **`Checkbox.vue`**
   - 基于 `reka-ui` 的 `CheckboxRoot` + `CheckboxIndicator`，统一 13x13 尺寸与珊瑚色选中背景。
4. **`Separator.vue`**
   - 基于 `reka-ui` 的 `Separator`，支持行内文本修饰（如“或使用以下方式”）。

### 5.2 动效 Composable

1. **`useTerminalPlayer.ts`（终端演示状态机）**
   - **剧本数据**：包含 `nmap`（服务探测）、`smb`（共享枚举）、`msf`（漏洞模块搜索）三套剧本。
   - **循环状态**：`typing`（逐字键入）→ `execute`（350ms 准备）→ `output`（逐行输出）→ `hold`（定格 1300ms）→ `clear`（清屏淡出 260ms）→ 下一剧本。
   - **运行优化**：
     - 使用 `requestAnimationFrame` 计算 `dt` 时间步进。
     - 监听 `document.visibilityState` / `pagehide`，后台标签页自动休眠，避免重新切回时动画暴冲。
     - 探测媒体查询 `prefers-reduced-motion: reduce`，匹配时降低运行速度 70% 并关闭过渡动画。
     - `onUnmounted` 自动调用 `cancelAnimationFrame`。
2. **`useModuleRail.ts`（横向无缝循环轨道）**
   - **无缝克隆**：在 DOM 挂载后通过卡片集合计算单轮物理偏移 `loopWidth`，动态克隆一套节点追加到末尾，克隆节点设置 `aria-hidden="true"` 与 `tabindex="-1"`。
   - **自动滚动**：每帧平滑累加 `scrollLeft`。
   - **交互暂停**：
     - 鼠标悬停（`pointerenter`）或键盘聚焦（`focusin`）时立即挂起自动滚动；
     - 离开后设定恢复延迟（鼠标离开 500ms，失焦 800ms）；
     - 捕获用户主动操作（`wheel`、`touchmove`、`keydown`），暂停自动步进并在 2800ms 后平滑恢复。
   - **观察器注销**：使用 `ResizeObserver` 响应窗口大小变化重新量算，`IntersectionObserver` 在模块区移出屏幕时暂停计算，组件卸载时完全注销。

---

## 6. 路由与渲染规则

### 6.1 路由定义

- **`/`（落地页）**：
  - 渲染策略：`prerender: true`，构建期直接生成静态 HTML，秒级首屏展现与 SEO 最佳化。
  - 核心模块：精简 Header、`HeroSection`、`TerminalBackdrop`、`FeaturedModule`、`ModuleRail`、Footer。
- **`/auth`（认证中心）**：
  - 渲染策略：`ssr: true`。
  - 模式切换：通过 URL 查询参数 `?mode=login|register|recover` 实现无刷新切换。
  - 交互行为：
    - 前端实时格式校验（邮箱格式正则、密码最小 6 位）。
    - 校验失败自动聚焦第一个错误输入框。
    - 提交后模拟 650ms 请求延迟并展示对应成功卡片（支持点击直接回到登录）。

---

## 7. 响应式布局与断点

严格复刻原型中经过验证的响应式规范：

| 视口断点 | 落地页表现 | 认证页表现 |
| :--- | :--- | :--- |
| **> 1500px** | 居中最大 1240px 容器，极佳留白与视野。 | 左右间距扩大至 100px，标语增大至 61px。 |
| **761px - 1100px** | 边距调整为 60px，模块卡片宽度自适应。 | 左右间距调整为 40px，左侧微调字体比例。 |
| **<= 760px (移动端)** | 顶部导航收缩至 73px；主推模块改为单列纵向排列；轨道支持自然触控滑动；终端字体自适应降至 12px。 | 隐藏左侧 `visual` 装饰区；表单区自适应居中；输入框高度提升至 48px，字号统一 16px 避免移动端输入聚焦放大。 |

---

## 8. 实施完成标准 (Definition of Done)

Phase 1 实施完成并验收需满足：
1. `cd web && npm run build` 构建成功无报错，全静态输出落地页 HTML。
2. 访问 `/` 首页：终端打字机自动输入与清屏循环正常，主推卡片微光跟随正常，横向模块架无缝循环且悬停/滑动正常暂停恢复。
3. 访问 `/auth` 页面：登录、注册、密码找回三种模式平滑切换，密码显隐开关、大写锁定提醒生效，表单校验与成功态预览完整呈现。
4. 移动端不同视口（360px、390px、768px）布局无水平溢出，样式与原型设计像素级对齐。
5. 所有代码完全类型安全，无 TypeScript 编译警告。
