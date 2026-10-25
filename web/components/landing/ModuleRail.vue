<script setup lang="ts">
import { ref } from 'vue'
import { useModuleRail } from '~/composables/useModuleRail'

const railRef = ref<HTMLElement | null>(null)
const { handleCardPointerMove } = useModuleRail(railRef)
</script>

<template>
  <div
    ref="railRef"
    class="module-rail"
    tabindex="0"
    role="region"
    aria-label="更多学习模块，横向滚动浏览"
  >
    <!-- 01 Linux -->
    <NuxtLink
      to="/auth"
      class="module-card"
      data-module="linux"
      data-level="foundation"
      style="--tone: #d6b188; --glow: #d6b188"
      aria-label="进入Linux 与系统基础模块"
      @pointermove="handleCardPointerMove"
    >
      <div class="module-art">
        <div class="card-topline">
          <span>01 / SYSTEM</span>
          <span class="level foundation">基础</span>
        </div>
        <img
          src="/images/linux.jpg"
          alt="电路板与系统硬件"
          width="900"
          height="600"
          loading="lazy"
          decoding="async"
        >
        <div class="art-code"><span class="quiet">~/labs/permissions</span>
<span class="bright">$ chmod u+x explore.sh</span>
→ ready to execute</div>
      </div>
      <div class="card-copy">
        <h3>Linux 与系统基础</h3>
        <p>权限、进程与系统里的第一条线索。</p>
        <div class="card-foot">
          <span>8 章节 · 3 次实践</span>

        </div>
      </div>
    </NuxtLink>

    <!-- 02 Web -->
    <NuxtLink
      to="/auth"
      class="module-card"
      data-module="web"
      data-level="advanced"
      style="--tone: #d3918f; --glow: #d3918f"
      aria-label="进入Web 漏洞分析模块"
      @pointermove="handleCardPointerMove"
    >
      <div class="module-art">
        <div class="card-topline">
          <span>02 / WEB SECURITY</span>
          <span class="level advanced">进阶</span>
        </div>
        <img
          src="/images/web.jpg"
          alt="计算机与应用工作台"
          width="900"
          height="600"
          loading="lazy"
          decoding="async"
        >
        <div class="art-code"><span class="quiet">REQUEST / RESPONSE</span>
<span class="bright">GET /api/profile HTTP/1.1</span>
<span>↳ trace the trust boundary</span></div>
      </div>
      <div class="card-copy">
        <h3>Web 漏洞分析</h3>
        <p>从请求出发，拆解应用的信任边界。</p>
        <div class="card-foot">
          <span>12 章节 · 6 次实践</span>

        </div>
      </div>
    </NuxtLink>

    <!-- 04 Network -->
    <NuxtLink
      to="/auth"
      class="module-card"
      data-module="network"
      data-level="foundation"
      style="--tone: #9ab0c7; --glow: #9ab0c7"
      aria-label="进入网络与协议模块"
      @pointermove="handleCardPointerMove"
    >
      <div class="module-art">
        <div class="card-topline">
          <span>04 / NETWORK</span>
          <span class="level foundation">基础</span>
        </div>
        <img
          src="/images/network.jpg"
          alt="网络服务器机房"
          width="900"
          height="600"
          loading="lazy"
          decoding="async"
        >
        <div class="art-code"><span class="quiet">PACKET CAPTURE</span>
<span class="bright">SYN → SYN, ACK → ACK</span>
192.0.2.10 → 192.0.2.20</div>
      </div>
      <div class="card-copy">
        <h3>网络与协议</h3>
        <p>看见流量如何到达，理解连接如何建立。</p>
        <div class="card-foot">
          <span>9 章节 · 4 次实践</span>

        </div>
      </div>
    </NuxtLink>

    <!-- 05 Reverse Engineering -->
    <NuxtLink
      to="/auth"
      class="module-card"
      data-module="reverse"
      data-level="advanced"
      style="--tone: #b69cdb; --glow: #b69cdb"
      aria-label="进入二进制与逆向分析模块"
      @pointermove="handleCardPointerMove"
    >
      <div class="module-art reverse-art">
        <div class="card-topline">
          <span>05 / REVERSE ENGINEERING</span>
          <span class="level advanced">进阶</span>
        </div>
        <div class="art-code"><span class="quiet">0000000000401126 &lt;main&gt;</span>
  <span class="bright">push   rbp</span>
  mov    rbp, rsp
> <span class="quiet">call   0x401050 &lt;verify&gt;</span></div>
      </div>
      <div class="card-copy">
        <h3>二进制与逆向分析</h3>
        <p>穿过函数与内存，理解程序真正做了什么。</p>
        <div class="card-foot">
          <span>14 章节 · 7 次实践</span>

        </div>
      </div>
    </NuxtLink>

    <!-- 06 Container Security -->
    <NuxtLink
      to="/auth"
      class="module-card"
      data-module="container"
      data-level="advanced"
      style="--tone: #87b8b1; --glow: #87b8b1"
      aria-label="进入容器安全与逃逸模块"
      @pointermove="handleCardPointerMove"
    >
      <div class="module-art container-art">
        <div class="card-topline">
          <span>06 / CONTAINER SECURITY</span>
          <span class="level advanced">进阶</span>
        </div>
        <svg
          class="diagram"
          viewBox="0 0 400 176"
          fill="none"
          aria-hidden="true"
          focusable="false"
        >
          <rect x="93" y="37" width="214" height="100" rx="8" stroke="currentColor" opacity=".4"/>
          <path d="M93 112H307" stroke="currentColor" opacity=".4"/>
          <rect x="113" y="54" width="45" height="40" rx="4" stroke="currentColor"/>
          <rect x="177" y="54" width="45" height="40" rx="4" stroke="currentColor"/>
          <rect x="241" y="54" width="45" height="40" rx="4" stroke="currentColor"/>
          <path d="M158 74H177 M222 74H241" stroke="currentColor" stroke-dasharray="2 3"/>
          <text x="170" y="129" fill="currentColor" font-size="9" font-family="monospace">HOST KERNEL</text>
          <text x="124" y="78" fill="currentColor" font-size="8" font-family="monospace">PID 1</text>
          <text x="187" y="78" fill="currentColor" font-size="8" font-family="monospace">CAPS</text>
          <text x="249" y="78" fill="currentColor" font-size="8" font-family="monospace">NS</text>
        </svg>
      </div>
      <div class="card-copy">
        <h3>容器安全与逃逸</h3>
        <p>从隔离机制深入容器的权限与边界。</p>
        <div class="card-foot">
          <span>11 章节 · 5 次实践</span>

        </div>
      </div>
    </NuxtLink>
  </div>
</template>
