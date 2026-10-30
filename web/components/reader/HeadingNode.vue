<script setup lang="ts">
import { ChevronDown } from 'lucide-vue-next'
import type { HeadingNode as OutlineNode } from '~/utils/reader-outline'
const props = defineProps<{
  node: OutlineNode
  activeId: string
  activePath: string[]
  folded: Set<string>
  controlsPrefix: string
}>()
defineEmits<{ select: [id: string]; toggle: [id: string] }>()
const childrenId = computed(() => `${props.controlsPrefix}-${props.node.id}`)
</script>

<template>
  <li class="heading-entry">
    <div class="heading-row">
      <a
        :href="`#${encodeURIComponent(node.id)}`"
        :aria-current="activeId === node.id ? 'location' : undefined"
        :class="{ 'contains-active': activeId !== node.id && activePath.includes(node.id) }"
        @click.prevent="$emit('select', node.id)"
        >{{ node.text }}</a
      >
      <button
        v-if="node.children.length"
        type="button"
        class="heading-toggle"
        :aria-label="`${folded.has(node.id) ? '展开' : '收起'}${node.text}的子目录`"
        :aria-expanded="!folded.has(node.id)"
        :aria-controls="childrenId"
        @click="$emit('toggle', node.id)"
      >
        <ChevronDown :size="14" :class="{ folded: folded.has(node.id) }" aria-hidden="true" />
      </button>
    </div>
    <ol
      v-if="node.children.length"
      v-show="!folded.has(node.id)"
      :id="childrenId"
      class="heading-children"
    >
      <HeadingNode
        v-for="child in node.children"
        :key="child.id"
        :node="child"
        :active-id="activeId"
        :active-path="activePath"
        :folded="folded"
        :controls-prefix="controlsPrefix"
        @select="$emit('select', $event)"
        @toggle="$emit('toggle', $event)"
      />
    </ol>
  </li>
</template>

<style scoped>
.heading-row {
  display: flex;
  align-items: center;
  min-width: 0;
}
.heading-row a {
  flex: 1;
  min-width: 0;
  padding: 8px 9px;
  color: var(--muted);
  text-decoration: none;
  font-size: 12px;
  line-height: 1.6;
  overflow-wrap: anywhere;
  border-radius: 5px;
}
.heading-row a:hover {
  background: var(--soft);
  color: var(--ink);
}
.heading-row a[aria-current='location'] {
  color: var(--brand);
  background: var(--brand-soft);
  font-weight: 600;
}
.heading-row a.contains-active {
  color: var(--ink);
}
.heading-toggle {
  flex-shrink: 0;
  display: grid;
  place-items: center;
  width: 26px;
  height: 30px;
  padding: 0;
  border: 0;
  border-radius: 5px;
  color: var(--muted);
  background: transparent;
  cursor: pointer;
}
.heading-toggle:hover {
  background: var(--soft);
  color: var(--ink);
}
.heading-toggle .folded {
  transform: rotate(-90deg);
}
.heading-children {
  list-style: none;
  margin: 2px 0 6px 9px;
  padding: 0 0 0 12px;
  border-left: 1px solid var(--line);
}
.heading-row a:focus-visible,
.heading-toggle:focus-visible {
  outline: 2px solid var(--focus);
  outline-offset: -2px;
}
</style>
