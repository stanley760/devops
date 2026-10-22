<script setup lang="ts">
import type { TagKind } from './types'

withDefaults(defineProps<{ kind?: TagKind }>(), { kind: 'default' })
</script>

<template>
  <span class="tag" :class="kind !== 'default' && `tag-${kind}`">
    <slot />
  </span>
</template>
