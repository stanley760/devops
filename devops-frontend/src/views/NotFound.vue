<script setup lang="ts">
import { useRouter } from 'vue-router'
import { Button } from '@/components/ui'

const router = useRouter()

function goHome(): void {
  void router.push('/')
}
</script>

<template>
  <div class="stage" style="display: flex; place-items: center; min-height: 100vh">
    <div class="m-glass" style="padding: 56px 64px; text-align: center; max-width: 520px">
      <div class="eyebrow">404 / NOT FOUND</div>
      <h1 style="margin: 12px 0 8px; font-size: 32px">页面不存在</h1>
      <p style="opacity: 0.72; margin: 0 0 28px; line-height: 1.7">
        你访问的页面不存在或已被移除，请检查地址是否正确，或返回首页继续操作。
      </p>
      <Button variant="primary" @click="goHome">返回首页</Button>
    </div>
  </div>
</template>
