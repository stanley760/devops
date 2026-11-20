import { useId, useState } from 'react'
import { Button } from '@/components/ui/button'
import { Field, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'

export interface SearchField {
  key: string
  label: string
  type?: 'input' | 'select'
  options?: Array<{ label: string; value: string }>
}

interface SearchFormProps {
  fields: SearchField[]
  value: Record<string, unknown>
  onSearch: () => void
  onReset: () => void
}

export function SearchForm({ fields, value, onSearch, onReset }: SearchFormProps) {
  const formId = useId()
  const [draft, setDraft] = useState<Record<string, unknown>>({ ...value })

  const applyDraft = () => {
    // 输入框空串不进入查询（后端空值剔除兜底）
    const cleaned = Object.fromEntries(Object.entries(draft).filter(([, v]) => v !== ''))
    for (const k of Object.keys(value)) delete value[k]
    Object.assign(value, cleaned)
  }

  return (
    <div className="flex flex-wrap items-end gap-3 rounded-md p-3">
      {fields.map((f) => (
        <Field key={f.key} className="min-w-40">
          <FieldLabel htmlFor={`${formId}-${f.key}`}>{f.label}</FieldLabel>
          {f.type === 'select' ? (
            <Select
              value={(draft[f.key] as string) ?? ''}
              onValueChange={(v) => setDraft((d) => ({ ...d, [f.key]: v }))}
            >
              <SelectTrigger><SelectValue placeholder="全部" /></SelectTrigger>
              <SelectContent>
                <SelectItem value="">全部</SelectItem>
                {f.options!.map((o) => (
                  <SelectItem key={o.value} value={o.value}>{o.label}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          ) : (
            <Input
              id={`${formId}-${f.key}`}
              value={(draft[f.key] as string) ?? ''}
              onChange={(e) => setDraft((d) => ({ ...d, [f.key]: e.target.value }))}
            />
          )}
        </Field>
      ))}
      <Button onClick={() => { applyDraft(); onSearch() }}>查询</Button>
      <Button
        variant="outline"
        onClick={() => {
          for (const k of Object.keys(draft)) draft[k] = ''
          setDraft({ ...draft })
          applyDraft()
          onReset()
        }}
      >
        重置
      </Button>
    </div>
  )
}
