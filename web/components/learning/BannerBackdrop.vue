<script setup lang="ts">
/** Stable across SSR, hydration and editor preview; RPG Hero vs. Dragon illustration. */
const props = defineProps<{ seed: string }>()
const identity = computed(() => {
  let hash = 2166136261
  for (const char of props.seed) hash = Math.imul(hash ^ char.charCodeAt(0), 16777619) >>> 0
  // Warm RPG fantasy palettes (Crimson Dragonfire, Paladin Amber Gold, Wyrm Emerald, Lava Bronze) - never purple
  return {
    color: ['#c2410c', '#b45309', '#0f766e', '#9a3412'][hash % 4],
    motif: (hash >>> 8) % 3
  }
})
</script>

<template>
  <div class="banner-backdrop" :style="{ '--banner-accent': identity.color }" aria-hidden="true">
    <svg viewBox="0 0 1200 240" preserveAspectRatio="xMidYMid slice" fill="none">
      <defs>
        <linearGradient id="rpg-fire" x1="955" y1="92" x2="770" y2="148" gradientUnits="userSpaceOnUse">
          <stop offset="0%" stop-color="#fef08a" stop-opacity="0.95" />
          <stop offset="38%" stop-color="#f97316" stop-opacity="0.88" />
          <stop offset="78%" stop-color="#dc2626" stop-opacity="0.65" />
          <stop offset="100%" stop-color="#991b1b" stop-opacity="0" />
        </linearGradient>
        <linearGradient id="rpg-shield" x1="768" y1="78" x2="812" y2="182" gradientUnits="userSpaceOnUse">
          <stop offset="0%" stop-color="#38bdf8" stop-opacity="0.9" />
          <stop offset="50%" stop-color="#fbbf24" stop-opacity="0.95" />
          <stop offset="100%" stop-color="#f97316" stop-opacity="0.25" />
        </linearGradient>
        <linearGradient id="rpg-dragon" x1="910" y1="18" x2="1160" y2="232" gradientUnits="userSpaceOnUse">
          <stop offset="0%" stop-color="currentColor" stop-opacity="0.9" />
          <stop offset="62%" stop-color="#7c2d12" stop-opacity="0.8" />
          <stop offset="100%" stop-color="#431407" stop-opacity="0.58" />
        </linearGradient>
        <linearGradient id="rpg-hero" x1="690" y1="92" x2="768" y2="215" gradientUnits="userSpaceOnUse">
          <stop offset="0%" stop-color="#d97706" stop-opacity="0.94" />
          <stop offset="100%" stop-color="#78350f" stop-opacity="0.88" />
        </linearGradient>
        <linearGradient id="rpg-fade" x1="480" y1="0" x2="680" y2="0" gradientUnits="userSpaceOnUse">
          <stop offset="0%" stop-color="#ffffff" stop-opacity="0" />
          <stop offset="100%" stop-color="#ffffff" stop-opacity="1" />
        </linearGradient>
        <mask id="rpg-scene-mask">
          <rect x="480" y="0" width="720" height="240" fill="url(#rpg-fade)" />
        </mask>
      </defs>

      <g mask="url(#rpg-scene-mask)">
        <!-- 远景：古老六芒星魔法阵与天穹满月光轮 -->
        <g class="rpg-runes" stroke="currentColor">
          <circle cx="985" cy="105" r="88" stroke-width="1.2" stroke-dasharray="6 4" />
          <circle cx="985" cy="105" r="72" stroke-width="1" />
          <polygon v-if="identity.motif !== 1" points="985,22 1056,146 914,146" stroke-width="0.9" />
          <polygon v-if="identity.motif !== 2" points="985,188 1056,64 914,64" stroke-width="0.9" />
        </g>

        <!-- 远景：魔龙火山群峰与古堡城垛剪影 -->
        <path
          d="M520 240 L605 172 L645 194 L708 134 L748 166 L812 116 L868 160 L936 86 L996 146 L1068 62 L1138 134 L1200 76 L1200 240 Z"
          fill="currentColor"
          fill-opacity="0.1"
        />
        <path
          d="M828 166 V124 H836 V132 H844 V124 H852 V132 H860 V124 H868 V166 Z"
          fill="currentColor"
          fill-opacity="0.18"
        />

        <!-- 恶龙巨翼 (双层龙翼与金色翼骨棱线) -->
        <path
          d="M1015 88 C1055 25, 1135 8, 1192 24 C1162 52, 1152 82, 1168 108 C1132 94, 1098 104, 1082 128 C1058 106, 1035 104, 1015 112 Z"
          fill="url(#rpg-dragon)"
          opacity="0.58"
        />
        <path
          d="M982 78 C1022 10, 1112 -6, 1182 12 C1140 44, 1124 78, 1142 110 C1098 92, 1062 104, 1044 134 C1018 108, 992 104, 968 116 Z"
          fill="url(#rpg-dragon)"
        />
        <path
          d="M982 78 C1030 42, 1090 36, 1142 110 M982 78 C1015 62, 1048 74, 1044 134"
          stroke="#fbbf24"
          stroke-width="1.3"
          stroke-opacity="0.4"
        />

        <!-- 恶龙躯干、狰狞龙首、背脊骨刺与盘踞龙尾 -->
        <path
          d="M952 92 C928 74, 906 66, 892 72 L870 62 L882 80 L862 86 L882 96 C898 102, 916 100, 930 112 C918 134, 915 162, 932 188 L905 204 L938 202 C956 218, 995 228, 1055 226 C1112 224, 1158 202, 1178 168 C1150 188, 1110 198, 1068 194 C1030 190, 1012 162, 1018 128 C1024 98, 988 86, 952 92 Z"
          fill="url(#rpg-dragon)"
        />
        <path d="M902 70 C922 46, 952 36, 976 42 C950 54, 932 64, 920 78 Z" fill="#7c2d12" />
        <path
          d="M944 82 L958 68 L962 86 L978 74 L980 94 L998 84 L996 105"
          stroke="currentColor"
          stroke-width="2.4"
          stroke-linejoin="round"
        />
        <polygon points="894,80 904,77 900,84" fill="#fef08a" />

        <!-- 恶龙烈焰吐息 (Dragon Breath) -->
        <path
          d="M872 90 C842 98, 812 108, 784 114 C815 124, 838 140, 790 156 C828 152, 855 132, 878 102 Z"
          fill="url(#rpg-fire)"
        />
        <path
          d="M874 92 C836 106, 804 122, 778 134"
          stroke="#fef08a"
          stroke-width="2.6"
          stroke-linecap="round"
          opacity="0.88"
        />

        <!-- 勇者魔法护盾壁垒 (Arcane Shield Barrier) -->
        <path
          d="M776 82 C798 106, 802 146, 780 176"
          stroke="url(#rpg-shield)"
          stroke-width="4"
          stroke-linecap="round"
        />
        <path
          d="M784 92 C802 112, 804 142, 786 166"
          stroke="#fef08a"
          stroke-width="1.5"
          stroke-dasharray="4 5"
          stroke-linecap="round"
        />

        <!-- 左侧勇者断崖基座 -->
        <path
          d="M615 240 L668 188 L758 176 L792 196 L775 240 Z"
          fill="currentColor"
          fill-opacity="0.22"
        />
        <path
          d="M645 240 L685 194 L756 180 L778 240 Z"
          fill="currentColor"
          fill-opacity="0.34"
        />

        <!-- 勇者剪影：红披风、羽冠头盔、高举圣剑与纹章盾 -->
        <path
          d="M724 134 C696 142, 672 162, 658 186 C684 180, 706 174, 726 164 Z"
          fill="#dc2626"
          fill-opacity="0.82"
        />
        <circle cx="734" cy="120" r="8.5" fill="url(#rpg-hero)" />
        <path
          d="M730 112 C718 104, 706 106, 698 114 C710 114, 720 115, 728 118 Z"
          fill="#f59e0b"
        />
        <path
          d="M724 130 L744 130 L750 160 L758 182 L748 184 L738 160 L726 184 L716 182 L724 156 Z"
          fill="url(#rpg-hero)"
        />
        <path
          d="M742 132 L756 116 L796 68 L802 64 L798 72 L760 120 Z"
          fill="#fef08a"
          stroke="#d97706"
          stroke-width="1.2"
        />
        <path d="M748 112 L764 124" stroke="#f59e0b" stroke-width="3" stroke-linecap="round" />
        <path
          d="M754 126 L770 122 L772 142 C772 152, 764 160, 756 164 C750 158, 748 148, 754 126 Z"
          fill="#f59e0b"
          stroke="#78350f"
          stroke-width="1.5"
        />

        <!-- 战场飞溅火星与 RPG 暴击符文粒子 -->
        <circle cx="812" cy="96" r="2.5" fill="#fef08a" />
        <circle cx="828" cy="142" r="2.2" fill="#f97316" />
        <circle cx="792" cy="76" r="1.8" fill="#38bdf8" />
        <circle cx="855" cy="72" r="2.2" fill="#fbbf24" />
        <circle cx="765" cy="172" r="2" fill="#fbbf24" />
        <polygon points="835,118 840,113 845,118 840,123" fill="#fef08a" opacity="0.88" />
        <polygon points="712,86 716,80 720,86 716,92" fill="#f59e0b" opacity="0.7" />
      </g>
    </svg>
  </div>
</template>

<style scoped>
.banner-backdrop {
  position: absolute;
  inset: 0;
  z-index: -2;
  pointer-events: none;
  color: var(--banner-accent);
  background:
    radial-gradient(
      ellipse 55% 95% at 82% 45%,
      color-mix(in srgb, var(--banner-accent) 18%, transparent),
      rgba(245, 158, 11, 0.06) 58%,
      transparent 82%
    ),
    linear-gradient(
      112deg,
      var(--paper) 38%,
      color-mix(in srgb, var(--banner-accent) 8%, #fffbeb) 100%
    );
}
:global([data-theme='midnight']) .banner-backdrop {
  background:
    radial-gradient(
      ellipse 55% 95% at 82% 45%,
      color-mix(in srgb, var(--banner-accent) 28%, transparent),
      rgba(180, 83, 9, 0.1) 58%,
      transparent 82%
    ),
    linear-gradient(
      112deg,
      var(--paper) 36%,
      color-mix(in srgb, var(--banner-accent) 16%, #141218) 100%
    );
}
svg {
  width: 100%;
  height: 100%;
  position: absolute;
  inset: 0;
}
.rpg-runes {
  opacity: 0.24;
}
@media (max-width: 640px) {
  svg {
    opacity: 0.32;
  }
}
</style>
