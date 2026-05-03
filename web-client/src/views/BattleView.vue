<template>
  <div class="battle-page">
    <header class="top-bar">
      <h1>战斗</h1>
      <button @click="router.push('/main')">返回主界面</button>
    </header>
    <div class="content">
      <p class="placeholder">战斗系统开发中...</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useRouter } from 'vue-router'
const router = useRouter()
</script>

<style scoped>
.battle-page {
  min-height: 100vh;
  background: var(--bg);
  color: var(--text);
}
.top-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 24px;
  border-bottom: 1px solid var(--border);
}
.top-bar h1 { margin: 0; font-size: 20px; }
.top-bar button {
  padding: 8px 14px;
  border-radius: 8px;
  border: 1px solid var(--border);
  background: var(--surface);
  color: var(--text);
  cursor: pointer;
}
.content {
  padding: 40px;
  text-align: center;
}
.placeholder {
  color: var(--muted);
  font-size: 16px;
}
</style>
