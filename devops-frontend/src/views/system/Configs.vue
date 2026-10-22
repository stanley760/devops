<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { configApi, type SysConfig, type ConfigSave } from '@/api/system'
import { useListPage, type ListPageState } from '@/composables/list'
import { useDictOptions } from '@/composables/options'
import { confirmAction } from '@/composables/confirm'
import { toastOk, toastError } from '@/composables/toast'
import type { Column } from '@/components/ui/types'
import { Field, TextInput, Select, Switch, Tag } from '@/components/ui'
import { ListPage, FormDialog, PermissionButton } from '@/components/business'

type MutList = ListPageState<SysConfig> & { afterMutate: (removedCount?: number) => Promise<void> }

const list = useListPage<SysConfig>({
  query: { config_name: '' },
  fetch: (p) => configApi.list(p),
  rowKey: 'id',
}) as MutList

const query = list.query as { config_name: string }

const columns: Column[] = [
  { key: 'configKey', title: '参数键', width: 200 },
  { key: 'configName', title: '参数名称', width: 160 },
  { key: 'configValue', title: '参数值', width: 220 },
  { key: 'type', title: '类型', width: 90 },
  { key: 'visible', title: '可见', width: 80 },
  { key: 'remark', title: '备注' },
  { key: 'actions', title: '操作', width: 140, className: 'col-actions' },
]

const typeOptions = [
  { value: 1, label: '内置' },
  { value: 2, label: '自定义' },
]

/**
 * 参数值按「键的语义」渲染不同控件，而不是一律自由文本。
 *
 * 这不是体验优化，是正确性要求：部分内置键的取值域由后端约束。例��
 * sys.captcha.type 必须是字典 sys_captcha_type 的值之一 —— 后端只在**读**时对非法值
 * 做兜底回落，写入非法值不报错，但登录验证码会静默退化成默认题型，很难排查。
 * 所以：
 *   - 布尔类键 → Switch（避免手打 "1"/"yes" 等后端 parseBool 不认的值）
 *   - 枚举类键 → 读字典下拉（选项来自 /dict-data/simple，与后端取值域同源）
 *   - 其余     → TextInput
 * 新增自定义键按「自由文本」处理。
 */
type ValueControl = 'bool' | 'dict' | 'text'

interface ControlSpec {
  control: ValueControl
  dictType?: string
}

/** 内置键 → 控件映射；字典编码与后端 service/captcha.go 的常量保持一致。 */
const CONTROL_MAP: Record<string, ControlSpec> = {
  'sys.account.registerUser': { control: 'bool' },
  'sys.captcha.enabled': { control: 'bool' },
  'sys.captcha.type': { control: 'dict', dictType: 'sys_captcha_type' },
}

const captchaTypeOptions = useDictOptions('sys_captcha_type')

const dialogOpen = ref(false)
const busy = ref(false)
const editingId = ref<string | null>(null)
/** 编辑中的原始键（参数键创建后不可改，用它决定渲染哪种控件） */
const editingKey = ref('')

function emptyForm() {
  return { config_key: '', config_name: '', config_value: '', type: 2, visible: true, remark: '' }
}

const form = reactive(emptyForm())

/** 当前编辑项的控件类型。 */
const currentControl = computed<ControlSpec>(() => CONTROL_MAP[editingKey.value] ?? { control: 'text' })

/** 枚举控件的选项。 */
const currentDictOptions = computed(() =>
  currentControl.value.dictType === 'sys_captcha_type' ? captchaTypeOptions.value : [],
)

/** 布尔值以 'true'/'false' 字符串与表单往返，提交时无需额外转换。 */
const boolValue = computed({
  get: () => form.config_value === 'true',
  set: (v: boolean) => {
    form.config_value = v ? 'true' : 'false'
  },
})

function openCreate(): void {
  editingId.value = null
  editingKey.value = ''
  Object.assign(form, emptyForm())
  dialogOpen.value = true
}

function openEdit(row: SysConfig): void {
  editingId.value = row.id
  editingKey.value = row.configKey
  Object.assign(form, {
    config_key: row.configKey,
    config_name: row.configName,
    config_value: row.configValue ?? '',
    type: row.type ?? 2,
    visible: row.visible !== false,
    remark: row.remark ?? '',
  })
  dialogOpen.value = true
}

async function submit(): Promise<void> {
  if (!form.config_name.trim()) {
    toastError(new Error('请填写参数名称'))
    return
  }
  if (!editingId.value && !form.config_key.trim()) {
    toastError(new Error('请填写参数键'))
    return
  }
  busy.value = true
  try {
    const body: ConfigSave = {
      config_name: form.config_name.trim(),
      config_value: form.config_value,
      type: Number(form.type) || 2,
      visible: form.visible,
      remark: form.remark,
    }
    if (editingId.value) {
      await configApi.update(editingId.value, body)
    } else {
      body.config_key = form.config_key.trim()
      await configApi.create(body)
    }
    toastOk(editingId.value ? '参数已更新' : '参数已创建')
    dialogOpen.value = false
    await list.load()
  } catch (e) {
    toastError(e)
  } finally {
    busy.value = false
  }
}

async function remove(row: SysConfig): Promise<void> {
  // 内置项被删除后只能靠重跑种子恢复，且 sys.captcha.enabled 这类键缺失会改变
  // 后端的默认安全行为（缺失 = 开启验证码），所以这里明确禁止删除 type=1。
  if (row.type === 1) {
    toastError(new Error('内置配置项不可删除，如需变更请直接修改参数值'))
    return
  }
  const ok = await confirmAction({ title: '确认删除', text: '删除后不可恢复，确认继续？', danger: true })
  if (!ok) return
  busy.value = true
  try {
    await configApi.remove(row.id)
    toastOk('已删除')
    await list.afterMutate(1)
  } catch (e) {
    toastError(e)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <ListPage title="参数配置" desc="运行时系统参数与开关" :list="list" :columns="columns" table-title="参数列表" row-key="id" min-width="960">
    <template #actions>
      <PermissionButton code="system:config:create" variant="primary" icon="ep:plus" label="新增参数" @click="openCreate" />
    </template>

    <template #search>
      <Field label="参数名称">
        <TextInput v-model="query.config_name" placeholder="参数名称" />
      </Field>
    </template>

    <template #cell-configKey="{ value }">
      <span class="mono">{{ value }}</span>
    </template>
    <template #cell-configValue="{ row }">
      <!-- 布尔类参数直接显示开关状态：'true'/'false' 原文对管理员没有直观含义 -->
      <Switch
        v-if="CONTROL_MAP[(row as SysConfig).configKey]?.control === 'bool'"
        :model-value="row.configValue === 'true'"
        disabled
      />
      <span
        v-else
        class="mono"
        :title="String(row.configValue)"
        style="display: inline-block; max-width: 200px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap"
      >{{ row.configValue || '-' }}</span>
    </template>
    <template #cell-type="{ value }">
      <Tag :kind="value === 1 ? 'violet' : 'blue'">{{ value === 1 ? '内置' : '自定义' }}</Tag>
    </template>
    <template #cell-visible="{ row }">
      <span>{{ row.visible === false ? '否' : '是' }}</span>
    </template>
    <template #cell-actions="{ row }">
      <PermissionButton code="system:config:update" variant="link" label="编辑" @click="openEdit(row)" />
      <PermissionButton
        v-if="row.type === 2"
        code="system:config:delete"
        variant="link"
        danger
        label="删除"
        @click="remove(row)"
      />
      <span v-else class="btn-link" style="opacity: 0.45; cursor: not-allowed" title="内置参数不可删除">删除</span>
    </template>
  </ListPage>

  <FormDialog :open="dialogOpen" :title="editingId ? '编辑参数' : '新增参数'" :busy="busy" @close="dialogOpen = false" @submit="submit">
    <Field label="参数键" required :help="editingId ? '创建后不可改' : undefined">
      <TextInput v-model="form.config_key" placeholder="如 sys.user.initPassword" :disabled="Boolean(editingId)" />
    </Field>
    <Field label="参数名称" required>
      <TextInput v-model="form.config_name" placeholder="参数名称" />
    </Field>

    <!-- 参数值：按参数键的语义切换控件。布尔用开关、枚举用字典下拉、其余自由文本。
         取值域由后端/字典约束，自由文本只能写非法值且不会立刻报错。 -->
    <Field
      v-if="currentControl.control === 'bool'"
      label="参数值"
      help="开关型参数，关闭等同于写入 false"
    >
      <Switch v-model="boolValue" />
    </Field>
    <Field
      v-else-if="currentControl.control === 'dict'"
      label="参数值"
      required
      :help="`可选取值来自字典 ${currentControl.dictType}，修改后立即生效`"
    >
      <Select
        v-model="form.config_value"
        :options="currentDictOptions"
        :placeholder="currentDictOptions.length ? '请选择' : '未加载到字典项，请确认拥有字典查看权限'"
      />
    </Field>
    <Field v-else label="参数值" help="自由文本，请勿写入非法值">
      <TextInput v-model="form.config_value" placeholder="参数值" />
    </Field>

    <Field v-if="!editingId" label="类型">
      <Select v-model="form.type" :options="typeOptions" />
    </Field>
    <Field v-else label="类型" help="内置标记由系统决定，不可修改">
      <Tag :kind="form.type === 1 ? 'violet' : 'blue'">{{ form.type === 1 ? '内置' : '自定义' }}</Tag>
    </Field>
    <Field v-if="!editingId" label="可见">
      <Switch v-model="form.visible" />
    </Field>
    <Field label="备注">
      <TextInput v-model="form.remark" placeholder="备注" />
    </Field>
  </FormDialog>
</template>
