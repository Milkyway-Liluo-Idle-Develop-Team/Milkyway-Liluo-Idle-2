<template>
  <div class="app-container">
    <header>
      <h1>游戏数据面板</h1>
      <div class="header-actions">
        <RouterLink class="icon-btn" to="/user" aria-label="用户中心">
          <svg class="header-icon" viewBox="0 0 24 24" fill="none" aria-hidden="true">
            <path d="M20 21a8 8 0 0 0-16 0" />
            <circle cx="12" cy="8" r="4" />
          </svg>
          <span class="sr-only">用户</span>
        </RouterLink>

        <button
          class="theme-switch"
          type="button"
          role="switch"
          :aria-checked="theme === 'dark'"
          :class="{ 'is-dark': theme === 'dark' }"
          @click="toggleTheme"
          :aria-label="themeAriaLabel"
        >
          <span class="thumb" aria-hidden="true">
            <svg v-if="theme === 'dark'" class="theme-icon" viewBox="0 0 24 24" fill="none">
              <path d="M21 14.2A8.2 8.2 0 0 1 9.8 3a6.8 6.8 0 1 0 11.2 11.2Z" />
            </svg>
            <svg v-else class="theme-icon" viewBox="0 0 24 24" fill="none">
              <circle cx="12" cy="12" r="4.5" />
              <path d="M12 2.5v2.5M12 19v2.5M21.5 12h-2.5M5 12H2.5" />
              <path d="M19.4 4.6l-1.8 1.8M6.4 17.6l-1.8 1.8" />
              <path d="M19.4 19.4l-1.8-1.8M6.4 6.4L4.6 4.6" />
            </svg>
          </span>
          <span class="sr-only">切换主题</span>
        </button>

        <button @click="fetchData" :disabled="loading" class="refresh-btn">
          <svg
            class="header-icon"
            :class="{ spinning: loading }"
            viewBox="0 0 24 24"
            fill="none"
            aria-hidden="true"
          >
            <path d="M20 6v6h-6" />
            <path d="M20 12a8 8 0 1 1-3.1-6.3" />
          </svg>
          <span class="sr-only">{{ loading ? '加载中' : '刷新数据' }}</span>
        </button>
      </div>
    </header>

    <LoadingError :loading="loading" :error="error" />

    <template v-if="!loading && !error">
      <ItemsSection :items="items" />
      <EventsSection :events="events" :rewardNameById="rewardNameById" />
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useActionsData } from '@/composables/UseActionsData'
import ItemsSection from '@/components/ItemsSection.vue'
import EventsSection from '@/components/EventsSection.vue'
import LoadingError from '@/components/LoadingError.vue'
import { useTheme } from '@/composables/useTheme'

const { items, events, loading, error, fetchData } = useActionsData()
const { theme, toggleTheme, themeAriaLabel } = useTheme()

const rewardNameById = computed<Record<string, string>>(() => {
  const map: Record<string, string> = {}
  for (const item of items.value ?? []) {
    if (!item?.id) continue
    map[item.id] = item.name || item.id
  }
  return map
})

onMounted(() => {
  fetchData()
})
</script>

<style scoped>
.app-container {
  max-width: 1400px;
  margin: 0 auto;
  padding: 20px;
  color: var(--text);
  font-family: ui-sans-serif, system-ui, -apple-system, 'Segoe UI', Roboto, Arial, 'PingFang SC',
    'Microsoft YaHei', sans-serif;
  min-height: 100vh;
  line-height: 1.5;
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
}

.app-container {
  background: radial-gradient(900px 520px at 10% -10%, rgba(0, 122, 204, 0.22), transparent 60%),
    radial-gradient(760px 520px at 92% -10%, rgba(0, 178, 148, 0.18), transparent 58%),
    linear-gradient(180deg, var(--bg), var(--bg-2));
}
header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 30px;
  border-bottom: 1px solid var(--border);
  padding-bottom: 15px;
  gap: 16px;
}
h1 {
  color: var(--text);
  margin: 0;
  font-size: 1.55rem;
  font-weight: 750;
  letter-spacing: 0.2px;
  background: linear-gradient(90deg, var(--brand), var(--brand-2));
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
}
.header-actions {
  display: flex;
  align-items: center;
  gap: 10px;
  --head-btn-h: 34px;
  --head-btn-w: 54px;
}
.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  white-space: nowrap;
  border: 0;
}

.header-icon {
  width: 18px;
  height: 18px;
  stroke: currentColor;
  stroke-width: 2;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.spinning {
  animation: spin 0.95s linear infinite;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}

.icon-btn {
  width: var(--head-btn-w);
  height: var(--head-btn-h);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  text-decoration: none;
  border: none;
  border-radius: 999px;
  background: var(--head-btn-bg);
  color: var(--text);
  cursor: pointer;
  box-shadow: var(--shadow-sm);
  transition: transform 0.15s ease, box-shadow 0.15s ease, filter 0.15s ease;
}

.icon-btn:hover {
  transform: translateY(-1px);
  filter: saturate(1.05);
  box-shadow: var(--shadow-md);
}

.icon-btn:active {
  transform: translateY(0px);
}

.icon-btn:focus-visible {
  outline: 2px solid rgba(0, 122, 204, 0.55);
  outline-offset: 3px;
}

.theme-switch {
  border: none;
  border-radius: 999px;
  width: 54px;
  height: var(--head-btn-h);
  padding: 0;
  background: var(--head-btn-bg);
  color: var(--text);
  cursor: pointer;
  box-shadow: var(--shadow-sm);
  display: inline-flex;
  align-items: center;
  justify-content: flex-start;
  position: relative;
  transition: background 0.18s ease, box-shadow 0.18s ease, filter 0.18s ease;
}

.theme-switch:hover {
  filter: saturate(1.05);
}

.theme-switch.is-dark {
  background: linear-gradient(
    135deg,
    color-mix(in srgb, var(--purple) 62%, var(--head-btn-bg)),
    color-mix(in srgb, var(--brand) 54%, var(--head-btn-bg))
  );
}

.theme-switch:focus-visible {
  outline: 2px solid rgba(0, 122, 204, 0.55);
  outline-offset: 3px;
}

.thumb {
  width: 28px;
  height: 28px;
  border-radius: 999px;
  margin-left: 3px;
  background: color-mix(in srgb, var(--surface) 88%, transparent);
  box-shadow: 0 10px 22px rgba(2, 6, 23, 0.22);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  transform: translateX(0px);
  transition: transform 220ms cubic-bezier(0.2, 0.85, 0.15, 1), background 0.18s ease;
}

.theme-switch.is-dark .thumb {
  transform: translateX(20px);
}

.theme-switch.is-dark .thumb {
  background: color-mix(in srgb, rgba(255, 255, 255, 0.16) 32%, var(--surface));
}

.theme-icon {
  width: 18px;
  height: 18px;
  stroke: currentColor;
  stroke-width: 2;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.refresh-btn {
  width: var(--head-btn-w);
  height: var(--head-btn-h);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(
    135deg,
    color-mix(in srgb, var(--brand) 82%, #000),
    color-mix(in srgb, var(--brand-2) 78%, #000)
  );
  border: 1px solid rgba(255, 255, 255, 0.12);
  padding: 0;
  border-radius: 999px;
  color: rgba(255, 255, 255, 0.96);
  font-weight: 750;
  cursor: pointer;
  transition: transform 0.15s ease, box-shadow 0.15s ease, filter 0.15s ease;
  box-shadow: 0 12px 30px rgba(0, 122, 204, 0.14), 0 18px 46px rgba(0, 178, 148, 0.12);
  text-shadow: 0 1px 0 rgba(0, 0, 0, 0.35);
}
.refresh-btn:hover {
  transform: translateY(-1px);
  filter: saturate(1.05);
  box-shadow: 0 16px 38px rgba(0, 122, 204, 0.18), 0 26px 64px rgba(0, 178, 148, 0.14);
}
.refresh-btn:active {
  transform: translateY(0px);
}
.refresh-btn:focus-visible {
  outline: 2px solid rgba(0, 122, 204, 0.55);
  outline-offset: 3px;
}
.refresh-btn:disabled {
  background: rgba(148, 163, 184, 0.16);
  color: rgba(255, 255, 255, 0.55);
  border-color: rgba(148, 163, 184, 0.18);
  box-shadow: none;
  cursor: not-allowed;
  transform: none;
}

@media (prefers-reduced-motion: reduce) {
  .theme-switch,
  .icon-btn,
  .refresh-btn {
    transition: none;
  }
  .theme-switch:hover,
  .theme-switch:active,
  .icon-btn:hover,
  .icon-btn:active,
  .refresh-btn:hover,
  .refresh-btn:active {
    transform: none;
  }
  .spinning {
    animation: none;
  }
}
</style>
