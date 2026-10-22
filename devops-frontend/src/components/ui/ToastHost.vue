<script setup lang="ts">
import { overlayEnter, overlayLeave } from '@/composables/motion'
import { useToastState } from '@/composables/toast'

const state = useToastState()

const enter = (el: Element, done: () => void) => overlayEnter(el, done, 'pop')
const leave = (el: Element, done: () => void) => overlayLeave(el, done, 'pop')
</script>

<template>
  <Teleport to="body">
    <div class="toast-host">
      <TransitionGroup :css="false" @enter="enter" @leave="leave">
        <div
          v-for="item in state.items"
          :key="item.id"
          class="toast m-float"
          :class="`is-${item.kind}`"
        >
          <span class="toast-dot" />
          {{ item.text }}
        </div>
      </TransitionGroup>
    </div>
  </Teleport>
</template>
