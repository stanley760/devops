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
      <div class="eyebrow">403 / NO ACCESS</div>
      <h1 style="margin: 12px 0 8px; font-size: 32px">无访问权限</h1>
      <p style="opacity: 0.72; margin: 0 0 28px; line-height: 1.7">
        当前账号未分配该菜单权限，请联系租户管理员。
      </p>
      <Button variant="secondary" @click="goHome">返回首页</Button>
    </div>
  </div>
</template>
