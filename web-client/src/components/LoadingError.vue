<template>
  <div v-if="loading" class="loading">{{ loadingText }}</div>
  <div v-else-if="error" class="error-msg">错误：{{ error }}</div>
</template>

<script setup lang="ts">
defineProps<{
  loading?: boolean
  error?: string
  loadingText?: string
}>()
</script>

<style scoped>
.loading {
  text-align: center;
  font-size: 1.1rem;
  margin-top: 100px;
  color: var(--text);
  padding: 14px 16px;
  border-radius: 14px;
  border: 1px solid color-mix(in srgb, var(--brand) 28%, transparent);
  background: color-mix(in srgb, var(--brand) 16%, transparent);
}
.error-msg {
  background: rgba(251, 113, 133, 0.12);
  color: rgba(254, 202, 202, 0.95);
  border: 1px solid rgba(251, 113, 133, 0.22);
  padding: 12px 14px;
  border-radius: 14px;
  margin-bottom: 20px;
}
</style>
