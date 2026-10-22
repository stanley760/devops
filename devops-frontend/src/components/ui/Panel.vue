<script setup lang="ts">
withDefaults(
  defineProps<{
    title?: string
    meta?: string
    flush?: boolean
  }>(),
  {
    title: '',
    meta: '',
    flush: false,
  },
)
</script>

<template>
  <section class="panel m-glass">
    <header class="panel-head">
      <div class="panel-title">
        <slot name="title">{{ title }}</slot>
      </div>
      <div v-if="meta" class="panel-meta">{{ meta }}</div>
      <div class="panel-tools">
        <slot name="tools" />
      </div>
    </header>
    <div class="panel-body" :class="flush && 'is-flush'">
      <slot />
    </div>
  </section>
</template>
