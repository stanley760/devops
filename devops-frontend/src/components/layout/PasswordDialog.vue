<script setup lang="ts">
/**
 * 修改密码：6-64 位校验 + 二次确认。
 * 后端改密后吊销全部会话，因此成功即 clearSession 并回登录页。
 */
import { reactive, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { Field, TextInput } from '@/components/ui'
import FormDialog from '@/components/business/FormDialog.vue'
import { authApi } from '@/api/auth'
import { clearSession } from '@/composables/auth'
import { toastOk, toastError } from '@/composables/toast'

const props = defineProps<{ open: boolean }>()
const emit = defineEmits<{ close: [] }>()

const router = useRouter()

const form = reactive({ old_password: '', new_password: '', confirm: '' })
const err = reactive({ old: '', new: '', confirm: '' })
const busy = ref(false)

function reset(): void {
  form.old_password = ''
  form.new_password = ''
  form.confirm = ''
  err.old = ''
  err.new = ''
  err.confirm = ''
}

watch(
  () => props.open,
  (open) => {
    if (open) reset()
  },
)

function validLen(v: string): boolean {
  return v.length >= 6 && v.length <= 64
}

async function submit(): Promise<void> {
  err.old = ''
  err.new = ''
  err.confirm = ''
  if (!validLen(form.old_password)) err.old = '原密码需 6-64 位'
  if (!validLen(form.new_password)) err.new = '新密码需 6-64 位'
  if (form.confirm !== form.new_password) err.confirm = '两次输入的新密码不一致'
  if (err.old || err.new || err.confirm) return

  busy.value = true
  try {
    await authApi.changePassword({ old_password: form.old_password, new_password: form.new_password })
    toastOk('密码已修改，请重新登录')
    emit('close')
    clearSession()
    await router.replace('/login')
  } catch (e) {
    toastError(e, '修改失败')
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <FormDialog :open="open" title="修改密码" :busy="busy" @close="emit('close')" @submit="submit">
    <Field label="原密码" required :error="err.old">
      <TextInput v-model="form.old_password" type="password" placeholder="当前登录密码" :invalid="!!err.old" />
    </Field>
    <Field label="新密码" required :error="err.new" help="6-64 位，改密后需重新登录" style="margin-top: 14px">
      <TextInput v-model="form.new_password" type="password" placeholder="新密码" :invalid="!!err.new" />
    </Field>
    <Field label="确认新密码" required :error="err.confirm" style="margin-top: 14px">
      <TextInput v-model="form.confirm" type="password" placeholder="再次输入新密码" :invalid="!!err.confirm" />
    </Field>
  </FormDialog>
</template>
