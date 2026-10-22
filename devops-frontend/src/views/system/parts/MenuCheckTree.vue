<script setup lang="ts">
import type { SysMenu } from '@/api/system'
import { Checkbox } from '@/components/ui'

const props = defineProps<{
  node: SysMenu
  checked: Record<string, boolean>
}>()

function walk(n: SysMenu, on: boolean): void {
  props.checked[n.id] = on
  for (const c of n.children ?? []) walk(c, on)
}

function toggle(on: boolean): void {
  walk(props.node, on)
}
</script>

<template>
  <div class="menu-check-node">
    <label class="menu-check-row">
      <Checkbox :model-value="!!checked[node.id]" @update:model-value="toggle" />
      <span>{{ node.name }}</span>
      <span v-if="node.permission" class="mono" style="opacity: 0.55; font-size: 12px">{{ node.permission }}</span>
    </label>
    <div v-if="node.children?.length" class="menu-check-children" style="margin-left: 18px">
      <MenuCheckTree v-for="child in node.children" :key="child.id" :node="child" :checked="checked" />
    </div>
  </div>
</template>
