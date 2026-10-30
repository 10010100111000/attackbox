<script setup lang="ts">
import ReferenceReader from '~/components/reader/ReferenceReader.vue'
import type { ReferenceReading } from '~/shared/references'
import { apiErrorMessage } from '~/utils/api'
definePageMeta({ layout: 'learning' })
const id = String(useRoute().params.id)
const { data, error } = await useFetch<{ data: ReferenceReading & { status: string } }>(
  `/api/admin/references/${id}/preview`,
  { key: `admin:reference-preview:${id}` }
)
if (error.value || !data.value)
  throw createError({
    statusCode: error.value?.statusCode || 503,
    message:
      error.value?.statusCode === 401
        ? '请先登录管理员账号'
        : error.value?.statusCode === 403
          ? '需要管理员权限'
          : apiErrorMessage(error.value, '预览暂时不可用，请检查导入结果')
  })
const document = data.value!.data
useSeoMeta({ title: `${document.meta.title} — 管理员预览`, robots: 'noindex, nofollow' })
</script>
<template>
  <ReferenceReader
    :key="document.meta.revision"
    :document="document"
    preview
    :status="document.status"
    :back-to="`/admin/references/${id}`"
    back-label="资料管理"
  />
</template>
