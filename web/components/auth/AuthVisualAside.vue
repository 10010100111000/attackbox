<script setup lang="ts">
import { AUTH_BG_DATA_URI } from './authBgData'

// Auth visual side panel presenting mission taglines and background illustration
// Source asset: /images/auth-explore.webp
// 内联 Base64 背景图，随首屏 HTML 同步交付与同步解码，消除跨网往返与 hydration 导致的闪现延迟
const bgImage = AUTH_BG_DATA_URI
</script>

<template>
  <aside class="visual" aria-label="学习与探索">
    <!-- 静态背景底图 -->
    <div class="visual-bg" aria-hidden="true">
      <img
        :src="bgImage"
        alt=""
        class="visual-bg-img"
        width="400"
        height="439"
        loading="eager"
        decoding="sync"
        fetchpriority="high"
      />
    </div>

    <!-- 品牌主张文案 -->
    <div class="visual-text" aria-hidden="true">
      <div class="visual-tag">STAY CURIOUS.</div>
      <h2>让好奇，<span>有迹可循。</span></h2>
      <p class="visual-subtitle">从一个问题开始，<br>在实践中找到自己的答案。</p>
    </div>

    <!-- 底部状态标语 -->
    <div class="visual-foot">
      <span>LEARN. EXPLORE. VERIFY.</span>
      <span><i></i>ATTACKBOX</span>
    </div>
  </aside>
</template>

<style scoped>
.visual {
  height: min(660px, calc(100svh - 160px));
  min-height: 520px;
  position: relative;
  border-radius: 18px;
  border: 1px solid rgba(255, 255, 255, 0.04);
  background:
    radial-gradient(ellipse at 60% 35%, rgba(71, 48, 36, 0.2), transparent 65%),
    linear-gradient(135deg, #19171b, #0e1015 70%);
  overflow: hidden;
  isolation: isolate;
}

.visual::after {
  content: '';
  position: absolute;
  inset: 0;
  z-index: -1;
  background: linear-gradient(0deg, #101116 3%, rgba(17, 17, 23, 0.6) 26%, rgba(21, 19, 23, 0.81) 48%, rgba(23, 20, 25, 0.33) 100%);
  pointer-events: none;
}

/* 静态背景底图：清晰且融入暗色氛围，无多余模糊与动画 */
.visual-bg {
  position: absolute;
  inset: 0;
  z-index: 0;
  overflow: hidden;
  pointer-events: none;
  user-select: none;
  background: radial-gradient(circle at 80% 85%, rgba(68, 42, 30, 0.18), transparent 50%);
}

.visual-bg-img {
  position: absolute;
  right: 0;
  bottom: 0;
  width: 66%;
  max-width: 440px;
  height: auto;
  opacity: 0.82;
  filter: blur(2.5px);
  pointer-events: none;
}

/* 居中主标语：恢复纯正无杂质的原型质感 */
.visual-text {
  position: absolute;
  left: 54px;
  right: 35px;
  top: 50%;
  transform: translateY(-49%);
  z-index: 2;
  max-width: 360px;
}

.visual h2 {
  font-size: 54px;
  line-height: 1.28;
  letter-spacing: -2px;
  font-weight: 550;
  margin: 0;
  color: #f0eeeb;
}

.visual h2 span {
  display: block;
  background: linear-gradient(100deg, #ffe1b7, #ffb493 55%, #de939b);
  -webkit-background-clip: text;
  background-clip: text;
  color: transparent;
}

.visual-tag {
  font: 9px/1 "SFMono-Regular", Consolas, monospace;
  letter-spacing: 2px;
  color: #a78675;
  margin-bottom: 25px;
  display: flex;
  align-items: center;
  gap: 10px;
}

.visual-tag::before {
  content: '';
  height: 1px;
  width: 24px;
  background: #8b6658;
}

.visual-subtitle {
  font-size: 12px;
  line-height: 1.9;
  color: #988b95;
  margin-top: 22px;
  margin-bottom: 0;
}

.visual-foot {
  position: absolute;
  bottom: 30px;
  left: 36px;
  right: 36px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  font: 8px "SFMono-Regular", Consolas, monospace;
  letter-spacing: 1px;
  color: #69616d;
  z-index: 2;
}

.visual-foot span:last-child {
  color: #aa8c78;
}

.visual-foot i {
  display: inline-block;
  width: 4px;
  height: 4px;
  background: #c39c78;
  border-radius: 50%;
  margin-right: 8px;
  vertical-align: 2px;
}

@media (min-width: 1500px) {
  .visual-text {
    left: 70px;
  }
  .visual h2 {
    font-size: 61px;
  }
  .visual-bg-img {
    width: 70%;
    max-width: 480px;
  }
}

@media (max-width: 1100px) {
  .visual-text {
    left: 34px;
    right: 20px;
  }
  .visual h2 {
    font-size: 43px;
  }
  .visual-subtitle {
    font-size: 11px;
  }
  .visual-foot {
    left: 25px;
    right: 25px;
  }
  .visual-bg-img {
    width: 72%;
    max-width: 350px;
  }
}

@media (max-width: 760px) {
  .visual {
    display: none;
  }
}

@media (max-height: 700px) and (min-width: 761px) {
  .visual {
    min-height: 470px;
    height: 100%;
  }
}
</style>
