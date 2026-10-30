<script setup lang="ts">
import { RefreshCw, ChartNoAxesCombined } from 'lucide-vue-next'
import type { AnalyticsReport } from '~/shared/analytics'
import { adPlacements } from '~/shared/advertising'
definePageMeta({ layout: 'admin' })
useHead({ title: '数据分析 — AttackBox' })
const days = ref('7')
const { data, pending, error, refresh } = await useLazyFetch<{ data: AnalyticsReport }>(
  '/api/admin/analytics',
  { key: 'admin:analytics', query: computed(() => ({ days: days.value })) }
)
const report = computed(() => data.value?.data)
const metrics = computed(() =>
  report.value
    ? [
        ['页面访问', report.value.totals.views],
        ['访客估算', report.value.totals.visitors],
        ['有效阅读', report.value.totals.reads],
        ['广告曝光', report.value.totals.impressions],
        ['广告点击', report.value.totals.clicks],
        ['新增账户', report.value.totals.newAccounts],
        ['完成章节', report.value.totals.completedChapters]
      ]
    : []
)
const trend = computed(() => {
  const records = report.value?.daily || [],
    length = Number(days.value)
  const today = new Date()
  today.setUTCHours(0, 0, 0, 0)
  return Array.from({ length }, (_, index) => {
    const date = new Date(today.getTime() - (length - 1 - index) * 86400000)
      .toISOString()
      .slice(0, 10)
    return { day: date, views: records.find(r => r.day === date)?.views || 0 }
  })
})
const peak = computed(() => Math.max(1, ...trend.value.map(r => r.views)))
const rate = (clicks: number, impressions: number) =>
  impressions ? `${((100 * clicks) / impressions).toFixed(1)}%` : '—'
const channel = (value: string) =>
  ({ search: '搜索引擎', referral: '外站推荐', campaign: '活动链接', direct: '直接／来源未知' })[
    value
  ] || value
</script>

<template>
  <AdminListPage title="数据分析" description="查看浏览器访问来源、内容阅读和广告效果。">
    <template #actions
      ><UiSelect
        v-model="days"
        aria-label="统计时间范围"
        :options="[
          { value: '7', label: '近 7 天' },
          { value: '30', label: '近 30 天' },
          { value: '90', label: '近 90 天' }
        ]"
      /><UiButton variant="secondary" size="sm" :disabled="pending" @click="refresh()"
        ><RefreshCw :size="14" aria-hidden="true" />刷新</UiButton
      ></template
    >
    <AdminDataLoading :pending="pending" :has-data="!!data" />
    <UiNotice v-if="error" variant="error">统计暂时无法读取，请重试。</UiNotice>
    <template v-else-if="report">
      <div class="metric-grid">
        <div v-for="metric in metrics" :key="metric[0]" class="panel metric">
          <span>{{ metric[0] }}</span
          ><strong>{{ metric[1] }}</strong>
        </div>
      </div>
      <section class="panel chart-panel">
        <div class="section-heading">
          <h2>访问趋势</h2>
          <span>按访问日期归属 · UTC</span>
        </div>
        <div
          v-if="report.totals.views"
          class="trend"
          role="img"
          :aria-label="`近 ${days} 天页面访问趋势`"
        >
          <div
            v-for="row in trend"
            :key="row.day"
            class="trend-day"
            :title="`${row.day}：${row.views} 次访问`"
          >
            <span :style="{ height: `${(row.views / peak) * 100}%` }" />
          </div>
        </div>
        <div v-if="report.totals.views" class="trend-labels" aria-hidden="true">
          <span>{{ trend[0]?.day }}</span
          ><span>最高 {{ peak }} 次／日</span><span>{{ trend.at(-1)?.day }}</span>
        </div>
        <UiEmptyState v-else title="还没有访问数据" description="启用后的浏览器访问会显示在这里。"
          ><template #icon><ChartNoAxesCombined :size="26" aria-hidden="true" /></template
        ></UiEmptyState>
      </section>
      <div class="analysis-grid">
        <section>
          <h2>访问来源</h2>
          <div class="panel table-wrap">
            <table>
              <thead>
                <tr>
                  <th>来源</th>
                  <th>访问／访客</th>
                  <th>有效阅读</th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="source in report.sources"
                  :key="`${source.source}:${source.medium}:${source.campaign}`"
                >
                  <td>
                    <strong>{{
                      source.source === '(direct)' ? '直接／来源未知' : source.source
                    }}</strong
                    ><span class="record-summary"
                      >{{ channel(source.channel)
                      }}{{ source.campaign ? ` · ${source.campaign}` : ''
                      }}{{ source.medium ? ` · ${source.medium}` : '' }}</span
                    >
                  </td>
                  <td>{{ source.views }} / {{ source.visitors }}</td>
                  <td>{{ source.reads }}</td>
                </tr>
              </tbody>
            </table>
            <UiEmptyState v-if="!report.sources.length" title="暂无来源数据" />
          </div>
        </section>
        <section>
          <h2>热门页面</h2>
          <div class="panel table-wrap">
            <table>
              <thead>
                <tr>
                  <th>页面</th>
                  <th>访问</th>
                  <th>有效阅读</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="item in report.pages" :key="item.path">
                  <td>
                    <NuxtLink :to="item.path" class="page-link" target="_blank" rel="noopener">{{
                      item.path
                    }}</NuxtLink>
                  </td>
                  <td>{{ item.views }}</td>
                  <td>{{ item.reads }}</td>
                </tr>
              </tbody>
            </table>
            <UiEmptyState v-if="!report.pages.length" title="暂无页面数据" />
          </div>
        </section>
      </div>
      <section class="ad-performance">
        <h2>广告效果</h2>
        <div class="panel table-wrap">
          <table>
            <thead>
              <tr>
                <th>广告</th>
                <th>位置</th>
                <th>曝光</th>
                <th>点击</th>
                <th>点击率</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="ad in report.advertisements" :key="`${ad.id}:${ad.placement}`">
                <td>{{ ad.name }}</td>
                <td>
                  {{ adPlacements.find(p => p.value === ad.placement)?.label || ad.placement }}
                </td>
                <td>{{ ad.impressions }}</td>
                <td>{{ ad.clicks }}</td>
                <td>{{ rate(ad.clicks, ad.impressions) }}</td>
              </tr>
            </tbody>
          </table>
          <UiEmptyState v-if="!report.advertisements.length" title="暂无广告展示记录" />
        </div>
      </section>
      <p class="analytics-note">
        有效阅读：正文可见累计 30 秒，并达到 25%
        阅读深度。访客按浏览器匿名标识估算；管理员访问不计入。注册与章节完成为同期总量，不代表来源归因。数据保留
        90 天，浏览器拦截可能导致少记，统计不作为结算凭据。
      </p>
    </template>
  </AdminListPage>
</template>

<style scoped>
.metric-grid {
  display: grid;
  grid-template-columns: repeat(7, minmax(0, 1fr));
  gap: 12px;
  margin-bottom: 24px;
}
.metric {
  padding: 20px;
  display: grid;
  gap: 12px;
}
.metric span {
  color: var(--muted);
  font-size: 12px;
}
.metric strong {
  font-size: 26px;
  font-weight: 650;
}
h2 {
  margin: 0 0 16px;
  font-size: 15px;
  font-weight: 650;
}
.chart-panel {
  padding: 24px;
  margin-bottom: 28px;
}
.section-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.section-heading span {
  color: var(--muted);
  font-size: 11px;
}
.trend {
  display: flex;
  align-items: end;
  gap: 4px;
  height: 160px;
  margin-top: 18px;
  border-bottom: 1px solid var(--line);
}
.trend-labels {
  display: flex;
  justify-content: space-between;
  color: var(--muted);
  font-size: 11px;
  margin-top: 10px;
}
.trend-day {
  height: 100%;
  flex: 1;
  display: flex;
  align-items: end;
}
.trend-day span {
  display: block;
  width: 100%;
  border-radius: 3px 3px 0 0;
  background: var(--brand);
  opacity: 0.75;
}
.analysis-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 24px;
}
.analysis-grid table {
  min-width: 0;
}
.page-link {
  color: var(--ink);
  font-size: 11px;
  overflow-wrap: anywhere;
}
.ad-performance {
  margin-top: 28px;
}
.analytics-note {
  color: var(--muted);
  font-size: 11px;
  line-height: 1.8;
  margin-top: 24px;
}
</style>
