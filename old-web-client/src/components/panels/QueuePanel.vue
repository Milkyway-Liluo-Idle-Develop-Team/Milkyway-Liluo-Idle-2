<template>
  <div v-if="items.length > 0" class="queue-bar">
    <button class="queue-toggle" type="button" @click="collapsed = !collapsed">
      队列 ({{ items.length }})
      <span class="queue-toggle-arrow">{{ collapsed ? '▲' : '▼' }}</span>
    </button>
    <div v-if="!collapsed" class="queue-list">
      <div
        v-for="item in items"
        :key="`queue-${item.index}`"
        class="queue-item"
        :class="{ current: item.is_current, past: item.index < index }"
      >
        <span class="queue-index">{{ item.index + 1 }}</span>
        <span class="queue-name">{{ item.name }}</span>
        <span v-if="item.remaining != null" class="queue-remaining">剩{{ item.remaining }}次</span>
        <span v-if="item.is_current" class="queue-badge">当前</span>
        <span v-else-if="item.index < index" class="queue-actions">
          <button
            class="queue-btn danger"
            type="button"
            title="删除"
            :disabled="loading"
            @click="$emit('remove', item.index)"
          >
            <IconClose />
          </button>
        </span>
        <span v-else-if="item.index > index" class="queue-actions">
          <button
            class="queue-btn"
            type="button"
            title="上移"
            :disabled="loading || item.index <= index"
            @click="$emit('swap', item.index, item.index - 1)"
          >
            <IconChevronUp />
          </button>
          <button
            class="queue-btn"
            type="button"
            title="下移"
            :disabled="loading || item.index >= items.length - 1"
            @click="$emit('swap', item.index, item.index + 1)"
          >
            <IconChevronDown />
          </button>
          <button
            class="queue-btn"
            type="button"
            title="置顶"
            :disabled="loading"
            @click="$emit('bringToFront', item.index)"
          >
            <IconArrowToTop />
          </button>
          <button
            class="queue-btn danger"
            type="button"
            title="删除"
            :disabled="loading"
            @click="$emit('remove', item.index)"
          >
            <IconClose />
          </button>
        </span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import IconChevronUp from '@/components/icons/IconChevronUp.vue'
import IconChevronDown from '@/components/icons/IconChevronDown.vue'
import IconArrowToTop from '@/components/icons/IconArrowToTop.vue'
import IconClose from '@/components/icons/IconClose.vue'
import type { QueueItem } from '@/types/GameplayResponse'

defineProps<{
  items: QueueItem[]
  index: number
  loading: boolean
}>()

defineEmits<{
  remove: [index: number]
  swap: [fromIndex: number, toIndex: number]
  bringToFront: [index: number]
}>()

const collapsed = ref(false)
</script>

<style scoped>
.queue-bar {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 8px 12px;
  background: color-mix(in srgb, var(--surface-2) 80%, transparent);
  border: 1px solid var(--border);
  border-radius: 10px;
  margin: 0 10px 10px;
}

.queue-toggle {
  display: flex;
  align-items: center;
  gap: 8px;
  background: transparent;
  border: none;
  color: var(--text);
  font-weight: 700;
  cursor: pointer;
  padding: 0;
}

.queue-toggle-arrow {
  font-size: 0.7rem;
  color: var(--muted);
}

.queue-list {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.queue-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 4px 8px;
  border-radius: 6px;
  font-size: 0.82rem;
}

.queue-item.current {
  background: color-mix(in srgb, var(--brand) 18%, transparent);
  border: 1px solid color-mix(in srgb, var(--brand) 40%, transparent);
}

.queue-item.past {
  opacity: 0.5;
}

.queue-index {
  min-width: 20px;
  text-align: center;
  color: var(--muted);
  font-weight: 700;
}

.queue-name {
  flex: 1;
}

.queue-badge {
  font-size: 0.65rem;
  padding: 1px 6px;
  border-radius: 999px;
  background: color-mix(in srgb, var(--brand) 30%, transparent);
  color: var(--text);
}

.queue-actions {
  display: flex;
  gap: 4px;
}

.queue-btn {
  width: 22px;
  height: 22px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 4px;
  border: 1px solid var(--border);
  background: color-mix(in srgb, var(--surface-3) 80%, transparent);
  color: var(--text);
  font-size: 0.75rem;
  cursor: pointer;
  padding: 0;
}

.queue-btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.queue-btn.danger {
  color: #ff6b6b;
}

.queue-remaining {
  font-size: 0.7rem;
  color: var(--muted);
  padding: 1px 6px;
  border-radius: 999px;
  background: color-mix(in srgb, var(--surface-3) 60%, transparent);
}
</style>
