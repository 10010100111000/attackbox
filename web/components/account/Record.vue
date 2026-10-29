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
