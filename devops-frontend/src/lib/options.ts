import { useCallback, useEffect, useState } from 'react'

const cache = new Map<string, Promise<unknown>>()

export function loadOptions<T>(key: string, loader: () => Promise<T[]>): Promise<T[]> {
  let p = cache.get(key) as Promise<T[]> | undefined
  if (!p) {
    p = loader()
    cache.set(key, p)
  }
  return p
}

export function invalidateOptions(key?: string): void {
  if (key) cache.delete(key)
  else cache.clear()
}

export function useOptions<T>(key: string, loader: () => Promise<T[]>): {
  data: T[]
  loading: boolean
  reload: () => void
} {
  const [data, setData] = useState<T[]>([])
  const [loading, setLoading] = useState(true)
  const [tick, setTick] = useState(0)

  useEffect(() => {
    let alive = true
    setLoading(true)
    loadOptions<T>(key, loader)
      .then((d) => {
        if (alive) setData(d)
      })
      .finally(() => {
        if (alive) setLoading(false)
      })
    return () => {
      alive = false
    }
  }, [key, tick])

  const reload = useCallback(() => {
    cache.delete(key)
    setTick((t) => t + 1)
  }, [key])

  return { data, loading, reload }
}
