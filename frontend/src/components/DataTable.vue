<template>
  <div class="space-y-3">
    <!-- 顶部过滤与操作栏 -->
    <div v-if="showSearch || $slots.actions" class="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3">
      <div v-if="showSearch" class="relative max-w-xs flex-1">
        <Search class="w-3.5 h-3.5 absolute left-3 top-1/2 -translate-y-1/2 text-zinc-400" />
        <input
          v-model="globalFilter"
          type="text"
          :placeholder="searchPlaceholder"
          class="w-full pl-8 pr-3 py-1.5 text-xs bg-white border border-zinc-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-zinc-900 transition-all font-sans"
        />
      </div>
      <div v-else class="flex-1"></div>

      <div class="flex items-center gap-2">
        <slot name="actions" />
      </div>
    </div>

    <!-- 表格容器 -->
    <div class="shadcn-card overflow-hidden">
      <div class="overflow-x-auto">
        <table class="w-full text-left border-collapse text-xs">
          <thead>
            <tr
              v-for="headerGroup in table.getHeaderGroups()"
              :key="headerGroup.id"
              class="border-b border-zinc-200 bg-zinc-50/70 text-[11px] text-zinc-500 select-none font-medium"
            >
              <th
                v-for="header in headerGroup.headers"
                :key="header.id"
                :class="[
                  'py-2.5 px-4',
                  header.column.getCanSort() ? 'cursor-pointer hover:text-zinc-900 transition-colors' : '',
                  header.column.columnDef.meta?.align === 'right' ? 'text-right' : ''
                ]"
                @click="header.column.getCanSort() ? header.column.toggleSorting() : null"
              >
                <div
                  :class="[
                    'flex items-center gap-1.5',
                    header.column.columnDef.meta?.align === 'right' ? 'justify-end' : ''
                  ]"
                >
                  <FlexRender
                    v-if="!header.isPlaceholder"
                    :render="header.column.columnDef.header"
                    :props="header.getContext()"
                  />
                  <span v-if="header.column.getCanSort()" class="text-zinc-400 inline-flex">
                    <ArrowUp v-if="header.column.getIsSorted() === 'asc'" class="w-3 h-3 text-zinc-900" />
                    <ArrowDown v-else-if="header.column.getIsSorted() === 'desc'" class="w-3 h-3 text-zinc-900" />
                    <ArrowUpDown v-else class="w-3 h-3 opacity-40 hover:opacity-100" />
                  </span>
                </div>
              </th>
            </tr>
          </thead>
          <tbody class="divide-y divide-zinc-200">
            <tr v-if="table.getRowModel().rows.length === 0">
              <td :colspan="columns.length" class="py-8 text-center text-zinc-400">
                暂无记录
              </td>
            </tr>
            <tr
              v-for="row in table.getRowModel().rows"
              :key="row.id"
              class="hover:bg-zinc-50/50 transition-colors"
            >
              <td
                v-for="cell in row.getVisibleCells()"
                :key="cell.id"
                :class="[
                  'py-2.5 px-4',
                  cell.column.columnDef.meta?.align === 'right' ? 'text-right' : ''
                ]"
              >
                <slot
                  :name="`cell-${cell.column.id}`"
                  :row="row.original"
                  :value="cell.getValue()"
                  :cell="cell"
                >
                  <FlexRender
                    :render="cell.column.columnDef.cell"
                    :props="cell.getContext()"
                  />
                </slot>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <!-- 分页底栏 -->
      <div
        v-if="table.getPageCount() > 1 || table.getFilteredRowModel().rows.length > pageSize"
        class="border-t border-zinc-200/80 px-4 py-2.5 flex items-center justify-between text-xs text-zinc-500 bg-zinc-50/40"
      >
        <div class="text-[11px]">
          共 <span class="font-medium text-zinc-700">{{ table.getFilteredRowModel().rows.length }}</span> 条数据，
          第 <span class="font-medium text-zinc-700">{{ table.getState().pagination.pageIndex + 1 }}</span> / {{ table.getPageCount() || 1 }} 页
        </div>
        <div class="flex items-center gap-1.5">
          <button
            @click="table.previousPage()"
            :disabled="!table.getCanPreviousPage()"
            class="px-2 py-1 border border-zinc-200 rounded text-zinc-700 hover:bg-white disabled:opacity-40 disabled:cursor-not-allowed transition-all text-xs cursor-pointer"
          >
            上一页
          </button>
          <button
            @click="table.nextPage()"
            :disabled="!table.getCanNextPage()"
            class="px-2 py-1 border border-zinc-200 rounded text-zinc-700 hover:bg-white disabled:opacity-40 disabled:cursor-not-allowed transition-all text-xs cursor-pointer"
          >
            下一页
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'
import {
  useVueTable,
  getCoreRowModel,
  getSortedRowModel,
  getFilteredRowModel,
  getPaginationRowModel,
  FlexRender,
} from '@tanstack/vue-table'
import { Search, ArrowUpDown, ArrowUp, ArrowDown } from 'lucide-vue-next'

const props = defineProps({
  data: {
    type: Array,
    required: true,
    default: () => [],
  },
  columns: {
    type: Array,
    required: true,
  },
  searchPlaceholder: {
    type: String,
    default: '搜索数据...',
  },
  pageSize: {
    type: Number,
    default: 15,
  },
  showSearch: {
    type: Boolean,
    default: true,
  },
})

const sorting = ref([])
const globalFilter = ref('')
const pagination = ref({
  pageIndex: 0,
  pageSize: props.pageSize,
})

const tableData = computed(() => props.data || [])

const table = useVueTable({
  get data() {
    return tableData.value
  },
  get columns() {
    return props.columns
  },
  state: {
    get sorting() {
      return sorting.value
    },
    get globalFilter() {
      return globalFilter.value
    },
    get pagination() {
      return pagination.value
    },
  },
  onSortingChange: (updaterOrValue) => {
    sorting.value =
      typeof updaterOrValue === 'function' ? updaterOrValue(sorting.value) : updaterOrValue
  },
  onGlobalFilterChange: (updaterOrValue) => {
    globalFilter.value =
      typeof updaterOrValue === 'function'
        ? updaterOrValue(globalFilter.value)
        : updaterOrValue
  },
  onPaginationChange: (updaterOrValue) => {
    pagination.value =
      typeof updaterOrValue === 'function'
        ? updaterOrValue(pagination.value)
        : updaterOrValue
  },
  getCoreRowModel: getCoreRowModel(),
  getSortedRowModel: getSortedRowModel(),
  getFilteredRowModel: getFilteredRowModel(),
  getPaginationRowModel: getPaginationRowModel(),
})
</script>
