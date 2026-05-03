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
      const response = await fetch(apiUrl('/api/actions'))
      if (!response.ok) throw new Error(`HTTP ${response.status}`)
      const data = await response.json()
      items.value = data.items || []
      events.value = data.events || []
      levelProduction.value = data.level_production || []
    } catch (err: any) {
      error.value = err.message || '网络请求失败'
    } finally {
      loading.value = false
    }
  }

  return { items, events, levelProduction, loading, error, fetchData }
}
