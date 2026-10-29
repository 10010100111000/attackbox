<script setup lang="ts">
import { BookOpen, Check, Flag } from 'lucide-vue-next'

const props = defineProps<{
  title: string
  to: string
  subtitle: string
  completed: number
  total: number
  complete: boolean
  updatedAt?: string | null
  kind: 'learning' | 'challenge'
}>()
const percent = computed(() =>
  props.total > 0
    ? Math.min(100, Math.max(0, Math.round((props.completed / props.total) * 100)))
    : 0
)
const unit = computed(() => (props.kind === 'learning' ? '章节' : '阶段'))
const date = computed(() => {
  if (!props.updatedAt) return ''
  const value = new Date(props.updatedAt)
  return Number.isNaN(value.getTime())
    ? ''
    : new Intl.DateTimeFormat('zh-CN', {
        month: 'short',
        day: 'numeric',
        timeZone: 'Asia/Shanghai'
      }).format(value)
})
</script>

<template>
  <NuxtLink
    :to="to"
    class="personal-record"
    :aria-label="`${title}，${complete ? '已完成' : '进行中'}`"
  >
    <span class="personal-record-icon" :class="{ complete }" aria-hidden="true">
      <Check v-if="complete" :size="19" />
      <BookOpen v-else-if="kind === 'learning'" :size="19" />
      <Flag v-else :size="19" />
    </span>
    <span class="personal-record-text">
      <strong>{{ title }}</strong>
      <span>{{ subtitle }}</span>
    </span>
    <span class="personal-record-progress">
      <span class="personal-record-count">
        <span>{{ completed }} / {{ total }} {{ unit }}</span>
        <span :class="{ 'personal-complete': complete }">
          {{ complete ? '已完成' : `${percent}%` }}
        </span>
      </span>
      <span
        class="personal-progress"
        role="progressbar"
        :aria-label="`${title}的${unit}进度`"
        :aria-valuenow="percent"
        :aria-valuemin="0"
        :aria-valuemax="100"
        :aria-valuetext="`已完成 ${completed} / ${total} ${unit}`"
      >
        <i :style="{ width: `${percent}%` }" />
      </span>
    </span>
    <time v-if="date" class="personal-record-date" :datetime="updatedAt!">
      {{ date }}
    </time>
  </NuxtLink>
</template>

<style scoped>
.personal-record {
  display: grid;
  grid-template-columns: 42px minmax(0, 1fr) 160px 68px;
  gap: 18px;
  align-items: center;
  padding: 20px 24px;
  color: var(--ink);
  text-decoration: none;
  transition: background 0.15s ease;
}

.personal-record + .personal-record {
  border-top: 1px solid var(--line);
}

.personal-record:hover {
  background: var(--hover);
}

.personal-record-icon {
  display: grid;
  place-items: center;
  width: 42px;
  height: 42px;
  border: 1px solid var(--line);
  border-radius: 11px;
  background: var(--soft);
  color: var(--ink);
  transition:
    transform 0.18s ease,
    border-color 0.18s ease;
}

.personal-record:hover .personal-record-icon {
  transform: scale(1.04);
  border-color: color-mix(in srgb, var(--ink) 24%, var(--line));
}

.personal-record-icon.complete {
  color: var(--success-ink);
  background: var(--success-bg);
  border-color: color-mix(in srgb, var(--success-ink) 25%, transparent);
}

.personal-record-text {
  display: grid;
  gap: 6px;
  min-width: 0;
}

.personal-record-text strong {
  font-size: 14px;
  line-height: 1.45;
  font-weight: 600;
  overflow-wrap: anywhere;
}

.personal-record-text > span {
  font-size: 12px;
  line-height: 1.5;
  color: var(--muted);
  overflow-wrap: anywhere;
}

.personal-record-progress {
  display: grid;
  gap: 8px;
}

.personal-record-count {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  color: var(--muted);
  font-size: 11px;
  font-variant-numeric: tabular-nums;
}

.personal-record-count .personal-complete {
  color: var(--success-ink);
  font-weight: 600;
}

.personal-record-date {
  text-align: right;
  font-size: 11px;
  color: var(--muted);
  white-space: nowrap;
  font-variant-numeric: tabular-nums;
}

@media (max-width: 1020px) {
  .personal-record {
    grid-template-columns: 38px minmax(0, 1fr) 130px;
    gap: 14px;
    padding: 20px;
  }

  .personal-record-date {
    display: none;
  }
}

@media (max-width: 760px) {
  .personal-record {
    grid-template-columns: 36px minmax(0, 1fr);
    padding: 20px 18px;
    gap: 12px;
  }

  .personal-record-icon {
    width: 34px;
    height: 34px;
  }

  .personal-record-progress {
    grid-column: 2;
    margin-top: 2px;
    gap: 8px;
  }

  .personal-record-text strong {
    font-size: 13px;
  }
}
</style>
