import { ref } from 'vue'
import { getJson } from '@/lib/api'
import type { GameplayData } from '@/types/GameplayResponse'

export function useGameplayData() {
  const data = ref<GameplayData | null>(null)
  const loading = ref(false)
  const error = ref('')

  const fetchData = async (): Promise<GameplayData | null> => {
    loading.value = true
    error.value = ''
    try {
      const res = await getJson<{ success: true; data: GameplayData }>('/api/gameplay', {
        credentials: 'include',
      })
      if (!res.ok) {
        error.value = res.error
        return null
      }
      data.value = res.data.data
      return data.value
    } finally {
      loading.value = false
    }
  }

  return { data, loading, error, fetchData }
}
