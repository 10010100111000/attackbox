<script setup lang="ts">
import type { MarkdownHeading } from '~/utils/markdown'
import { buildHeadingTree, headingPath } from '~/utils/reader-outline'
const props = withDefaults(
  defineProps<{ headings: MarkdownHeading[]; activeId?: string; label?: string }>(),
  { activeId: '', label: '正文目录' }
)
defineEmits<{ select: [id: string] }>()
const tree = computed(() => buildHeadingTree(props.headings))
const activePath = computed(() => headingPath(tree.value, props.activeId))
const folded = ref(new Set<string>())
const controlsPrefix = useId()
function toggle(id: string) {
  if (folded.value.has(id)) folded.value.delete(id)
  else folded.value.add(id)
}
watch(
  () => props.headings,
  headings => {
    const ids = new Set(headings.map(heading => heading.id))
    folded.value = new Set([...folded.value].filter(id => ids.has(id)))
  }
)
</script>

<template>
  <nav v-if="tree.length" :aria-label="label" class="heading-tree">
    <ol class="heading-roots">
      <ReaderHeadingNode
        v-for="node in tree"
        :key="node.id"
        :node="node"
        :active-id="activeId"
        :active-path="activePath"
        :folded="folded"
        :controls-prefix="controlsPrefix"
        @select="$emit('select', $event)"
        @toggle="toggle"
      />
    </ol>
  </nav>
</template>

<style scoped>
.heading-roots {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  gap: 3px;
}
</style>
