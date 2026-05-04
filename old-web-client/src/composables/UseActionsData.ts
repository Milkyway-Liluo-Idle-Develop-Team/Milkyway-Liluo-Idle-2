import { ref } from 'vue'
import type { Item, Event } from '@/types/ActionResponse'
import { apiUrl } from '@/lib/api'

export function useActionsData() {
  const items = ref<Item[]>([])
  const events = ref<Event[]>([])
  const levelProduction = ref<number[]>([])
  const loading = ref(false)
  const error = ref('')

  const fetchData = async () => {
    loading.value = true
    error.value = ''
    try {
      const response = await fetch(apiUrl('/api/v1/game/config'))
      if (!response.ok) throw new Error(`HTTP ${response.status}`)
      const data = await response.json()
      items.value = data.actions?.items || []
      events.value = data.actions?.events || []
      const csv = data.level_curve_csv || ''
      const lines = csv.trim().split(/\r?\n/)
      const curve: number[] = []
      for (let i = 1; i < lines.length; i++) {
        const parts = lines[i].split(',')
        if (parts.length >= 2) {
          const v = parseFloat(parts[1].trim())
          if (!isNaN(v)) curve.push(v)
        }
      }
      levelProduction.value = curve
    } catch (err: any) {
      error.value = err.message || '网络请求失败'
    } finally {
      loading.value = false
    }
  }

  return { items, events, levelProduction, loading, error, fetchData }
}
