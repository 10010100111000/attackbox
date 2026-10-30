<script setup lang="ts">
withDefaults(defineProps<{ title: string; description?: string }>(), { description: '' })
</script>

<template>
  <div class="admin-list-page">
    <header class="page-heading">
      <div class="heading-copy">
        <h1>{{ title }}</h1>
        <p v-if="description">{{ description }}</p>
      </div>
      <div v-if="$slots.actions" class="heading-actions"><slot name="actions" /></div>
    </header>
    <slot />
  </div>
</template>

<style>
/* Slot content shares a list layout, explicitly bounded by this component root.
   Controls keep their own UiButton/UiInput styles. */
.admin-list-page {
  width: 100%;
  color: var(--ink);
}
.admin-list-page > .page-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 24px;
  margin-bottom: 26px;
}
.admin-list-page .heading-copy {
  min-width: 0;
}
.admin-list-page .heading-copy h1 {
  margin: 0;
  font-size: 24px;
  line-height: 1.35;
  font-weight: 750;
}
.admin-list-page .heading-copy p {
  margin: 8px 0 0;
  font-size: 13px;
  line-height: 1.6;
  color: var(--muted);
}
.admin-list-page .heading-actions {
  display: flex;
  align-items: center;
  flex-shrink: 0;
  gap: 10px;
}
.admin-list-page .filters {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 18px;
  font-size: 12px;
}
.admin-list-page .count {
  margin-left: auto;
}
.admin-list-page .muted,
.admin-list-page .record-summary {
  color: var(--muted);
}
.admin-list-page .panel {
  background: var(--paper);
  border: 1px solid var(--line);
  border-radius: 12px;
  box-shadow: var(--surface-highlight), var(--card-shadow);
}
.admin-list-page .table-wrap {
  overflow-x: auto;
}
.admin-list-page table {
  width: 100%;
  min-width: 720px;
  border-collapse: collapse;
  text-align: left;
  font-size: 12px;
}
.admin-list-page th,
.admin-list-page td {
  padding: 16px 20px;
  border-bottom: 1px solid var(--line);
  vertical-align: middle;
}
.admin-list-page th {
  background: var(--soft);
  color: var(--muted);
  font-size: 11px;
  font-weight: 500;
}
.admin-list-page tr:last-child td {
  border-bottom: 0;
}
.admin-list-page .record-title {
  color: var(--ink);
  font-size: 14px;
  font-weight: 600;
  text-decoration: none;
}
.admin-list-page .record-title:hover {
  text-decoration: underline;
}
.admin-list-page .record-summary {
  display: block;
  font-size: 11px;
  margin-top: 5px;
}
.admin-list-page .actions {
  width: 1%;
  text-align: right;
  white-space: nowrap;
}
.admin-list-page .row-actions,
.admin-list-page .pagination > div {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}
.admin-list-page .pagination {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: 20px;
  font-size: 12px;
}
</style>
