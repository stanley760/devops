<script setup lang="ts">
import { settleConfirm, useConfirmState } from '@/composables/confirm'
import Modal from './Modal.vue'

const state = useConfirmState()
</script>

<template>
  <Modal
    :open="state.open"
    :title="state.title"
    :danger="state.danger"
    :confirm-text="state.confirmLabel"
    @confirm="settleConfirm(true)"
    @close="settleConfirm(false)"
  >
    {{ state.text }}
  </Modal>
</template>
