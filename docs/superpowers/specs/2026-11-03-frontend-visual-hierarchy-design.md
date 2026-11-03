# AttackBox 前端视觉与空间层次升级设计规范 (Carbon & Signal Orange)

- **日期**: 2026-11-03
- **视觉定位**: Teenage Engineering / Nothing / Linear Carbon 工业级设计语言（95% 纯粹冷调无彩色黑白灰 + 5% 高锐度信号橙点缀 + 五层光学纵深）
- **覆盖范围**: 全局设计 Token 系统、主布局外壳（`landing.vue` / `learning.vue`）、Landing 落地页、学习目录与横幅（`/learn`）、模块详情页（`/learn/modules/[slug]`）、挑战列表（`/challenges`）、参考资料（`/references`）、讨论占位页（`/discuss`）

---

## 1. 核心问题与设计目标

### 1.1 现有痛点
1. **色温昏黄催眠**：全站采用低饱和土铜棕（`#dfb086` / `#9c623e`）叠加偏黄暖灰文字与底色（`#f2f0ed` / `#f3f2ee`），在光学上接近 2700K 夜间护眼模式，缺乏对比度与精神感。
2. **背景粗糙杂乱**：
   - `layouts/learning.vue` 使用固定 `viewBox="0 0 1440 900"` 的缩放 SVG 网格与粗脉冲折线，在大屏上拉伸失真。
   - `components/landing/HeroSection.vue` 使用 4px 周期粗扫描线，显得粗糙。
   - `components/learning/BannerBackdrop.vue` 使用了与安全实验室定位割裂的「勇者斗恶龙」卡通剪影插画。
3. **缺乏 Z 轴空间层次**：筛选器、Tab 栏、搜索框与卡片全部平铺在同一平面（`--paper`），并使用完全相同的 `1px solid var(--line)` 全包围实线框，没有「内嵌凹槽（Well）」与「悬浮表面（Elevated Card）」的物理纵深反差。

### 1.2 设计目标
- **彻底洗去昏黄感，拒绝 AI 廉价渐变**：采用 95% 纯粹冷调锌黑/钛白无彩色基底（Achromatic Zinc/Carbon），搭配 5% 工业信号橙（Signal Orange `#f97316` / `#ea580c`）作为精准视觉锚点（指示灯、活动标尺、核心焦点）。
- **极简纯静深空背景**：移除缩放网格与粗扫描线，采用双源深空径向聚光（顶部冷白银穹顶柔光 + 极微弱信号橙氛围光）配合 SVG `feTurbulence` 胶片级微噪点纹理。
- **五层光学纵深（5-Tier Elevation System）**：通过下沉式凹槽容器（Level -1）、主画布（Level 0）、悬浮渐变卡片（Level +1）、顶光源高光棱边（Specular Top Rim）与多层漫反射阴影建立立体空间。

---

## 2. 全局色彩与光学 Token 规范 (`assets/css/main.css` & `brand-experience.css` & `learning-shell.css`)

### 2.1 暗色模式 (`[data-theme='midnight']`)
| 层级 / 语义 | 变量名 | 目标值 | 设计说明 |
| :--- | :--- | :--- | :--- |
| **Level 0 主画布** | `--bg` | `#09090b` | 纯粹深邃锌黑（Zinc 950），彻底去除暖棕底色 |
| **Level -1 下沉凹槽** | `--surface-well` | `rgba(4, 4, 6, 0.72)` | 用于工具栏、分类胶囊槽、空状态底座，配内阴影 |
| **凹槽内阴影** | `--well-shadow` | `inset 0 1px 2px rgba(0, 0, 0, 0.8), 0 1px 0 rgba(255, 255, 255, 0.05)` | 塑造物理下沉感 |
| **Level +1 卡片底色** | `--paper` | `#121215` | 哑光石墨灰，与主画布拉开清晰明度差 |
| **Level +2 内嵌块** | `--soft` | `#1c1c21` | 胶囊、徽章、次级块表面 |
| **悬浮态表面** | `--hover` | `#18181d` | 交互悬停表面 |
| **主文字** | `--ink` | `#fafafa` | 高反差纯净冷白，瞬间提亮视觉 |
| **正文文字** | `--body` | `#d4d4d8` | Zinc 300，清晰易读 |
| **次要文字** | `--muted` | `#8e8e96` | 纯正中性锌灰，不偏黄不偏紫 |
| **基础分割线** | `--line` | `rgba(255, 255, 255, 0.085)` | 克制的 1px 结构线 |
| **高光边框** | `--line-strong` | `rgba(255, 255, 255, 0.15)` | 悬浮卡片外框 |
| **工业信号橙（主点缀）** | `--brand` | `#f97316` | Teenage Engineering 风格高锐度信号橙 |
| **信号橙微光底** | `--brand-soft` | `rgba(249, 115, 22, 0.12)` | 仅用于关键徽章/指示器背景 |
| **信号橙高光边** | `--brand-border` | `rgba(249, 115, 22, 0.42)` | 激活态/悬停态精密边框 |
| **主按钮** | `--btn-primary-bg` / `fg` | `#fafafa` / `#09090b` | 高反差纯白实体按键 + 顶部亮棱 |
| **卡片光学表面** | `--card-surface` | `linear-gradient(180deg, rgba(255, 255, 255, 0.045) 0%, rgba(255, 255, 255, 0.005) 100%), var(--paper)` | 垂直微光泽模拟顶部受光 |
| **顶光源高光棱边** | `--surface-highlight` | `inset 0 1px 0 0 rgba(255, 255, 255, 0.13)` | 1px 顶部金属高光 |
| **卡片复合阴影** | `--card-shadow` | `0 1px 2px rgba(0, 0, 0, 0.5), 0 14px 32px -8px rgba(0, 0, 0, 0.75)` | 双层景深阴影 |
| **卡片悬停阴影** | `--card-hover-shadow` | `inset 0 1px 0 0 rgba(255, 255, 255, 0.22), 0 20px 42px -10px rgba(0, 0, 0, 0.85), 0 0 24px -6px rgba(249, 115, 22, 0.18)` | 悬停时唤醒顶部亮边与微弱信号橙环境光 |
| **深空双源聚光背景** | `--backdrop-glow` | `radial-gradient(ellipse 75% 420px at 50% -10%, rgba(255, 255, 255, 0.065), rgba(249, 115, 22, 0.035) 48%, transparent 80%)` | 顶部冷银白主光 + 极微弱信号橙边缘光 |

### 2.2 浅色模式 (`[data-theme='graphite']`)
- **主画布 (`--bg`)**：`#f4f4f5`（纯净冷锌灰 Zinc 100，告别旧版发黄的 `#f3f2ee`）
- **下沉凹槽 (`--surface-well`)**：`#e8e8ec` + `inset 0 1px 2px rgba(0, 0, 0, 0.06)`
- **卡片表面 (`--paper`)**：`#ffffff`（纯白悬浮卡片，与 `#f4f4f5` 画布形成干净的明度阶梯）
- **主文字 (`--ink`)**：`#09090b`（纯正曜石黑）
- **次要文字 (`--muted`)**：`#63636b`
- **工业信号橙 (`--brand`)**：`#ea580c`（深信号橙，保证浅色背景下 WCAG AA 对比度）
- **深空聚光 (`--backdrop-glow`)**：`radial-gradient(ellipse 75% 380px at 50% -10%, rgba(24, 24, 27, 0.045), rgba(234, 88, 12, 0.03) 50%, transparent 80%)`

### 2.3 自动化测试契约兼容保留
为确保现有测试套件 100% 通过：
1. `ui.test.js` 断言了 `main.css` 中基础回退变量的字符串字面量（如 `--bg-app: #090a0e;`、`--accent-coral: #ffaf88;` 等），在 `:root` 中保留这些回退声明，同时在业务组件中统一消费升级后的核心语义变量（`--bg`、`--paper`、`--ink`、`--brand`、`--surface-well`、`--card-surface`）。
2. `landing-browser.test.mjs` 断言了 `.auth-layout` 在 `midnight` 下的计算颜色（`rgb(242, 240, 237)`、`rgb(21, 21, 27)`、`rgb(242, 232, 223)`），因此 `brand-experience.css` 对 `.auth-layout` 保持精确隔离，而 `.landing-layout` 与 `.learning-layout` 使用全新的曜石黑 + 信号橙视觉系统。
3. `reader-layout-browser.test.mjs` 断言了 `--reader-ink` 的计算颜色（`rgb(39, 38, 43)` 与 `rgb(220, 224, 232)`），保持 `--reader-ink` 不变。

---

## 3. 布局与核心页面升级设计

### 3.1 平台主布局与纯静深空背景 (`layouts/learning.vue`)
- **移除 SVG 网格与折线**：删除 `<svg class="backdrop-circuit">` 及其所有 `.circuit-beam` / `.circuit-node` 动画。
- **双层光学背景 (`.learning-backdrop`)**：
  1. `.backdrop-spotlight`：呈现顶部穹顶冷白与微弱信号橙的交织径向聚光。
  2. `.backdrop-noise`：使用极轻量内联 SVG `feTurbulence` 噪点纹理（`opacity: 0.024`），赋予暗色画布细腻的阳极氧化铝/哑光碳纤维质感。
- **导航栏 (`components/SiteHeader.vue` & `LearningHeader.vue`)**：
  - 保持 `56px` 高度与全宽对齐契约；
  - 底部边框升级为渐隐高光边，当前激活导航项（`.main-nav a.active`）使用信号橙底边指示器（`2px` 锐利横条 + 微弱底部光晕）。

### 3.2 Landing 落地页 (`layouts/landing.vue`, `HeroSection.vue`, `ModuleCard.vue`, `pages/index.vue`)
- **HeroSection 舞台化纵深**：
  - 保留 `TerminalBackdrop.vue` 及其内部样式以通过单元测试，将终端输出文字色调通过外层滤镜与径向景深暗角遮罩融入纯黑深空背景，去除粗糙的 `repeating-linear-gradient` 扫描线。
  - 在大标题上方增加工业风状态铭牌（如带信号橙呼吸灯圆点的胶囊标签 `SYSTEM // LIVE ENVIRONMENT` 风格视觉引导），大标题采用高反差纯白 `#fafafa` 搭配锐利的冷银—信号橙微渐变字尾（`.warm`）。
  - 「开始学习」主按钮采用纯白高反差实体表面 + 1px 顶边亮棱 + 悬停信号橙环境背光。
- **模块展示区 (`pages/index.vue` & `ModuleCard.vue` & `ModuleRail.vue`)**：
  - Featured Module 采用「左侧精密几何/封面暗房舞台 + 右侧高对比度排版」的双栏悬浮容器，外框具备顶边高光棱线。
  - `ModuleRail.vue` 与 `ModuleCard.vue` 的卡片色调从旧版粉棕/紫灰改为克制的工业色阶（信号橙 `#f97316`、冷钛白 `#e4e4e7`、青钢灰 `#71717a`、翠绿指示色 `#10b981`、冰钢蓝 `#38bdf8`），去除插图区与文字区之间生硬的实线切割，改用径向光泽过渡。

### 3.3 学习中心与横幅 (`pages/learn/index.vue`, `HomeCarousel.vue`, `BannerBackdrop.vue`)
- **推荐横幅插画重构 (`BannerBackdrop.vue`)**：
  - 彻底替换原有的「勇者斗恶龙」奇幻插画，改为符合 Teenage Engineering / 安全实验室美学的**精密等轴测拓扑雷达与硬件刻度阵列 SVG**（保留 `--banner-accent` 样式绑定与 `<svg>` 结构以通过 `learn-home-browser.test.mjs`）。
- **继续学习横幅 (`.recent-learning`)**：
  - 升级为左侧带信号橙垂直能量条、内部采用哑光石墨底色与高对比度进度槽的仪表盘式卡片。
- **工具栏凹槽化 (`.toolbar` & `.filters`)**：
  - 将 `.filters` 分类按钮组包裹在下沉式凹槽轨道（`background: var(--surface-well); box-shadow: var(--well-shadow); padding: 4px; border-radius: 10px`）中；
  - 激活的 `.filter-btn.active` 成为凹槽内浮起的高反差实体滑块，未激活项无多余外框，彻底消除满屏碎边框。
- **模块卡片网格 (`.cards` & `.card`)**：
  - 调整 `artThemes` 生成式背景：以深邃冷碳灰为基底，仅在右上角透出克制的单点径向微光（信号橙 / 冷银 / 钢蓝 / 矿物青），右上角序号胶囊 `.art-index-pill` 采用 `monospace` 精密工业铭牌样式（如 `01`）。
  - 卡片底栏 `.card-bottom` 采用高对比度状态文字，悬停时卡片整体上浮 `3px`，顶部 1px 棱边亮起。

### 3.4 挑战、参考资料、讨论与模块详情页 (`pages/challenges/index.vue`, `pages/references/index.vue`, `pages/discuss.vue`, `pages/learn/modules/[slug]/index.vue`)
- **挑战页 (`challenges/index.vue`)**：
  - 顶部 `.head-badge` 采用下沉凹槽内嵌信号橙图标设计；筛选栏 `.filters` 统一放入下沉式凹槽容器；挑战卡片顶部增加精细的难度色标指示与顶部高光棱线。
- **参考资料页 (`references/index.vue`)**：
  - 将原本平淡的单线列表 `.reference-item` 升级为具有独立悬浮光学表面（`--card-surface`）、顶边高光棱线与左侧带框图标底座的层叠卡片列表，拉开条目之间的空气感（`gap: 12px`）。
- **讨论页 (`discuss.vue`)**：
  - `.discuss-card` 升级为深空聚光舞台容器，顶部带信号橙微光点缀与金属质感边框。
- **模块详情页 (`learn/modules/[slug]/index.vue`)**：
  - `.hero-section` 升级为带顶部线性高光与径向微光的独立立体面板；
  - 章节列表 `.chapter-list` 采用外层下沉凹槽包裹、内部 `.chapter-row` 悬浮卡片化的双层结构，左侧 `.chapter-number` 采用工业仪表盘等宽字体铭牌。

---

## 4. 验证与测试计划
1. **样式规范审计**：运行 `node --test tests/styles.test.js`，确保无任何重复选择器定义。
2. **UI 与组件契约测试**：运行 `node --test tests/ui.test.js tests/landing.test.js tests/learning-ui.test.js`，确保所有文件结构、Token 回退与链接契约 100% 通过。
3. **构建与类型检查**：验证 Nuxt 样式编译与页面渲染无报错。
