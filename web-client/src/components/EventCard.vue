<template>
  <div class="card">
    <div class="card-header">
      <div class="title-wrap">
        <div class="title">{{ display(event.name) }}</div>
        <div class="meta">
          <span class="card-id">#{{ event.id }}</span>
          <span class="sub">{{ event.map || '未知地图' }}</span>
        </div>
      </div>
      <span class="badge" :class="event.type" :title="event.type" aria-label="事件类型">
        <span class="abbr">{{ typeAbbr }}</span>
        <span class="full">{{ typeSuffix }}</span>
      </span>
    </div>
    <div class="card-body">
      <div class="kv">
        <span class="k">描述</span>
        <span class="v multiline">{{ display(truncate(event.description, 90)) }}</span>
      </div>
      <div class="kv">
        <span class="k">所需技能</span>
        <span class="v">{{ event.need_skill || '无' }}</span>
      </div>
      <div v-if="event.loop_time" class="kv">
        <span class="k">循环时间</span>
        <span class="v">{{ event.loop_time }} 秒</span>
      </div>
      <div class="kv">
        <span class="k">经验</span>
        <span class="v">{{ event.experience || 0 }}</span>
      </div>
      <div v-if="event.rewards && event.rewards.length" class="kv rewards-row">
        <span class="k">奖励</span>
        <div class="rewards">
          <span v-for="reward in event.rewards" :key="reward.id" class="reward-chip">
            {{ rewardName(reward.id) }} × {{ reward.num ?? reward.value ?? 0 }}
          </span>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { Event } from '@/types/ActionResponse'

const props = defineProps<{ event: Event; rewardNameById: Record<string, string> }>()

const truncate = (text: string, length: number) => {
  if (!text) return ''
  return text.length > length ? text.slice(0, length) + '...' : text
}

const display = (value: unknown) => {
  const str = String(value ?? '').trim()
  return str ? str : '--'
}

const rewardName = (id: string) => {
  const str = String(id ?? '').trim()
  if (!str) return '--'
  return props.rewardNameById?.[str] || str
}

const toPrefixAbbr = (text: string) => {
  const normalized = String(text ?? '').trim()
  if (!normalized || normalized === '--') return '--'

  const chars = Array.from(normalized)
  const take = Math.min(2, chars.length)
  return chars.slice(0, take).join('').toUpperCase()
}

const typeText = computed(() => display(props.event.type))
const typeAbbr = computed(() => {
  const raw = String(props.event.type ?? '').trim()
  const mapped: Record<string, string> = {
    loop: 'LO',
    upgrade: 'UP',
    instant: 'IN',
  }
  return mapped[raw] || toPrefixAbbr(typeText.value)
})
const typeSuffix = computed(() => {
  const text = typeText.value
  const abbr = typeAbbr.value
  if (text === '--' || abbr === '--') return ''

  const textChars = Array.from(text)
  const abbrChars = Array.from(abbr)
  return textChars.slice(abbrChars.length).join('')
})
</script>

<style scoped>
.card {
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.06), rgba(255, 255, 255, 0.03));
  border: 1px solid var(--border);
  border-radius: 18px;
  overflow: hidden;
  box-shadow: var(--shadow-sm);
  transition: transform 0.16s ease, box-shadow 0.16s ease, border-color 0.16s ease;
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
}
.card:hover {
  transform: translateY(-3px);
  box-shadow: var(--shadow-md);
  border-color: rgba(59, 130, 246, 0.4);
}
.card-header {
  background: linear-gradient(90deg, rgba(34, 211, 238, 0.18), rgba(59, 130, 246, 0.22));
  padding: 12px 14px;
  display: flex;
  justify-content: flex-start;
  align-items: center;
  border-bottom: 1px solid rgba(148, 163, 184, 0.14);
  gap: 12px;
}
.title-wrap {
  flex: 1;
  min-width: 0;
}
.title {
  font-size: 1.02rem;
  font-weight: 800;
  letter-spacing: 0.2px;
  color: var(--text);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  text-rendering: geometricPrecision;
}
.meta {
  margin-top: 2px;
  display: flex;
  gap: 10px;
  align-items: center;
  min-width: 0;
}
.card-id {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New',
    monospace;
  font-weight: 700;
  font-size: 0.86rem;
  color: color-mix(in srgb, var(--brand-2) 92%, transparent);
  letter-spacing: 0.2px;
  font-variant-numeric: tabular-nums;
}
.sub {
  color: var(--muted-2);
  font-size: 0.86rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.badge {
  margin-left: auto;
  background: linear-gradient(
    135deg,
    color-mix(in srgb, var(--brand) 28%, var(--surface-2)),
    color-mix(in srgb, var(--brand-2) 22%, var(--surface-2))
  );
  border: none;
  padding: 4px 10px;
  border-radius: 999px;
  font-size: 0.8rem;
  text-transform: uppercase;
  color: var(--text);
  font-weight: 800;
  letter-spacing: 0.3px;
  white-space: nowrap;
  display: inline-flex;
  align-items: center;
  justify-content: flex-end;
  overflow: hidden;
  max-width: 56px;
  transition: max-width 0.28s ease, background 0.18s ease, box-shadow 0.18s ease;
}
.abbr {
  flex: 0 0 auto;
}
.full {
  max-width: 0px;
  overflow: hidden;
  white-space: nowrap;
  margin-left: 0px;
  opacity: 0;
  transform: translateX(4px);
  transition: max-width 0.28s ease, opacity 0.16s ease, transform 0.28s ease;
}
.badge:hover {
  max-width: 220px;
  box-shadow: 0 10px 22px rgba(2, 6, 23, 0.12);
}
.badge:hover .full {
  max-width: 180px;
  opacity: 1;
  transform: translateX(0px);
}
.badge.loop {
  background: color-mix(in srgb, var(--success) 22%, var(--surface-2));
  color: var(--text);
}
.badge.upgrade {
  background: color-mix(in srgb, var(--warning) 24%, var(--surface-2));
  color: var(--text);
}
.badge.instant {
  background: color-mix(in srgb, var(--purple) 22%, var(--surface-2));
  color: var(--text);
}
.card-body {
  padding: 14px;
  font-size: 0.92rem;
  color: var(--text);
  display: grid;
  gap: 10px;
}
.kv {
  display: grid;
  grid-template-columns: 88px 1fr;
  align-items: center;
  gap: 10px;
}
.k {
  color: var(--muted-2);
  font-weight: 650;
  letter-spacing: 0.2px;
}
.v {
  color: var(--muted);
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.v.multiline {
  white-space: normal;
  overflow: visible;
}
.rewards {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.rewards-row {
  margin-top: 4px;
}
.reward-chip {
  display: inline-flex;
  align-items: center;
  padding: 4px 10px;
  border-radius: 999px;
  border: none;
  background: color-mix(in srgb, var(--brand) 20%, var(--surface-2));
  color: var(--text);
  font-size: 0.84rem;
  font-weight: 700;
  letter-spacing: 0.2px;
}

@media (prefers-reduced-motion: reduce) {
  .card {
    transition: none;
  }
  .card:hover {
    transform: none;
  }
  .badge {
    transition: none;
  }
  .badge:hover {
    max-width: 56px;
    box-shadow: none;
  }
  .full {
    transition: none;
  }
  .badge:hover .full {
    max-width: 0px;
    opacity: 0;
    transform: translateX(4px);
  }
}

@media (max-width: 420px) {
  .kv {
    grid-template-columns: 76px 1fr;
  }
}
</style>
