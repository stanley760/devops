<script setup lang="ts">
withDefaults(
  defineProps<{
    label?: string
    required?: boolean
    error?: string
    help?: string
  }>(),
  {
    label: '',
    required: false,
    error: '',
    help: '',
  },
)
</script>

<template>
  <div class="field">
    <div v-if="label" class="field-label">
      {{ label }}<span v-if="required" class="req">*</span>
    </div>
    <slot />
    <div v-if="error" class="field-error">{{ error }}</div>
    <div v-else-if="help" class="field-help">{{ help }}</div>
  </div>
</template>
