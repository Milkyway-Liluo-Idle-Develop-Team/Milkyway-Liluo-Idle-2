<template>
  <div class="card">
    <div class="card-header">
      <div class="title-wrap">
        <div class="title">{{ display(item.name) }}</div>
        <div class="meta">
          <span class="card-id">#{{ item.id }}</span>
        </div>
      </div>
      <span class="badge" :title="item.classification" aria-label="分类">
        <span class="abbr">{{ classificationAbbr }}</span>
        <span class="full">{{ classificationSuffix }}</span>
      </span>
    </div>
    <div class="card-body">
      <div v-if="item.tool || item.equipment || item.upgradable" class="tags">
        <span v-if="item.tool" class="tag tag-tool">工具</span>
        <span v-if="item.equipment" class="tag tag-equipment">装备</span>
        <span v-if="item.upgradable" class="tag tag-upgradable">可升级</span>
      </div>
      <div v-if="item.tool" class="kv">
        <span class="k">工具类型</span>
        <span class="v">{{ displayOrItemName(item.tool_details?.tool_type) }}</span>
      </div>
      <div v-if="item.equipment" class="kv">
        <span class="k">装备类型</span>
        <span class="v">{{ displayOrItemName(item.equipment_details?.type) }}</span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { Item } from '@/types/ActionResponse'

const props = defineProps<{ item: Item }>()

const display = (value: unknown) => {
  const str = String(value ?? '').trim()
  return str ? str : '--'
}

const displayOrItemName = (value: unknown) => {
  const str = String(value ?? '').trim()
  if (str) return str
  const name = String(props.item?.name ?? '').trim()
  return name ? name : '--'
}

const toPrefixAbbr = (text: string) => {
  const normalized = String(text ?? '').trim()
  if (!normalized || normalized === '--') return '--'

  const chars = Array.from(normalized)
  const firstChar = chars[0] ?? ''
  const isAscii = firstChar.charCodeAt(0) <= 0x007f
  const take = isAscii ? 2 : 2
  return chars.slice(0, Math.min(take, chars.length)).join('').toUpperCase()
}

const classificationText = computed(() => display(props.item.classification))
const classificationAbbr = computed(() => toPrefixAbbr(classificationText.value))
const classificationSuffix = computed(() => {
  const text = classificationText.value
  const abbr = classificationAbbr.value
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
  border-color: color-mix(in srgb, var(--brand) 65%, transparent);
}
.card-header {
  background: linear-gradient(90deg, rgba(34, 211, 238, 0.2), rgba(59, 130, 246, 0.18));
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
.badge {
  margin-left: auto;
  background: linear-gradient(
    135deg,
    color-mix(in srgb, var(--brand) 28%, var(--surface-2)),
    color-mix(in srgb, var(--brand-2) 22%, var(--surface-2))
  );
  border: none;
  padding: 4px 10px 4px 10px;
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
.card-body {
  padding: 14px;
  font-size: 0.92rem;
  color: var(--text);
  --label-w: 88px;
  --kv-gap: 10px;
  display: grid;
  gap: 10px;
}
.tags {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.tag {
  display: inline-flex;
  align-items: center;
  padding: 4px 10px;
  border-radius: 999px;
  border: none;
  background: color-mix(in srgb, var(--surface-2) 92%, var(--surface-3) 8%);
  color: var(--text);
  font-size: 0.84rem;
  font-weight: 800;
  letter-spacing: 0.2px;
}
.tag-tool {
  background: color-mix(in srgb, var(--brand) 18%, var(--surface-2));
}
.tag-equipment {
  background: color-mix(in srgb, var(--brand-2) 16%, var(--surface-2));
}
.tag-upgradable {
  background: color-mix(in srgb, var(--warning) 16%, var(--surface-2));
}
.kv {
  display: grid;
  grid-template-columns: var(--label-w) 1fr;
  align-items: center;
  gap: var(--kv-gap);
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

@media (max-width: 420px) {
  .card-body {
    --label-w: 76px;
  }
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
</style>
