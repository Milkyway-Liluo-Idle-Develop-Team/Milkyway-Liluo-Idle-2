<template>
  <section class="section">
    <h2>物品 (Items) <span class="count">{{ items.length }}</span></h2>
    <div class="card-grid">
      <ItemCard v-for="item in items" :key="item.id" :item="item" />
    </div>
  </section>
</template>

<script setup lang="ts">
import type { Item } from '@/types/ActionResponse'
import ItemCard from './ItemCard.vue'

defineProps<{ items: Item[] }>()
</script>

<style scoped>
.section {
  margin-bottom: 50px;
}
.section h2 {
  font-size: 1.35rem;
  margin-bottom: 20px;
  color: var(--text);
  display: flex;
  align-items: center;
  gap: 10px;
  letter-spacing: 0.2px;
}
.count {
  background: rgba(255, 255, 255, 0.06);
  border: 1px solid var(--border);
  padding: 3px 10px;
  border-radius: 999px;
  font-size: 0.95rem;
  color: var(--muted);
}
.card-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
  gap: 16px;
}
</style>
