<template>
  <div class="market-panel">
    <section class="market-hero">
      <div>
        <h2>玩家市场</h2>
      </div>
      <div class="market-actions">
        <button class="market-refresh" type="button" :disabled="loading" @click="emit('refresh')">
          {{ loading ? '刷新中…' : '刷新市场' }}
        </button>
      </div>
    </section>

    <p v-if="error" class="error">{{ error }}</p>

    <template v-else-if="snapshot">
      <section class="market-summary">
        <article class="summary-card">
          <span class="summary-label">钱包余额</span>
          <strong>{{ snapshot.currency.icon }} {{ formatDisplayValue(snapshot.walletBalance) }}</strong>
        </article>
        <article class="summary-card">
          <span class="summary-label">活跃挂单</span>
          <strong>{{ formatDisplayValue(snapshot.activeListings) }}</strong>
        </article>
        <article class="summary-card">
          <span class="summary-label">24h 成交额</span>
          <strong>{{ snapshot.currency.icon }} {{ formatDisplayValue(snapshot.totalVolume24h) }}</strong>
        </article>
        <article class="summary-card">
          <span class="summary-label">最后刷新</span>
          <strong>{{ formatDateTime(snapshot.lastUpdatedAt) }}</strong>
        </article>
      </section>

      <section class="market-grid">
        <article class="market-card">
          <div class="card-head">
            <h3>发布挂单</h3>
            <span class="head-tag">{{ sellCandidates.length }} 个可售物品</span>
          </div>

          <form class="sell-form" @submit.prevent="submitCreate">
            <label class="field">
              <span>物品</span>
              <select
                v-model="draft.itemId"
                class="market-field"
                :disabled="loading || !sellCandidates.length"
              >
                <option v-for="item in sellCandidates" :key="item.itemId" :value="item.itemId">
                  {{ item.itemName }} · 持有 x{{ formatDisplayValue(item.quantity) }}
                </option>
              </select>
            </label>

            <div class="field-row">
              <label class="field field--half">
                <span>数量</span>
                <div class="field-input-group field-input-group--inside">
                  <input
                    v-model.trim="draft.quantity"
                    class="market-field market-field--with-btn"
                    type="text"
                    inputmode="decimal"
                    placeholder="如 1.2K"
                    :disabled="loading || !selectedSellCandidate"
                    @input="sanitizeDraftQuantityInput"
                    @blur="normalizeDraftInput('quantity')"
                  />
                  <button
                    class="inline-btn inline-btn--inside inline-btn--max"
                    type="button"
                    :disabled="loading || !selectedSellCandidate"
                    @click="fillDraftMaxQuantity"
                  >
                    MAX
                  </button>
                </div>
              </label>
              <label class="field field--half">
                <span>单价</span>
                <div class="field-input-group">
                  <input
                    v-model.trim="draft.unitPrice"
                    class="market-field"
                    type="text"
                    inputmode="decimal"
                    placeholder="如 2.5K"
                    :disabled="loading || !selectedSellCandidate"
                    @input="sanitizeDraftInput('unitPrice')"
                    @blur="normalizeDraftInput('unitPrice')"
                  />
                </div>
              </label>
            </div>

            <p v-if="selectedSellCandidate" class="form-hint">
              <span class="form-hint-row">
                分类：{{ selectedSellCandidate.classification }} · 当前最多可上架
                {{ formatDisplayValue(selectedSellCandidate.quantity) }}
              </span>
              <span class="form-hint-row">
                数量 {{ formatDisplayValue(draftExact.quantity) }} · 单价
                {{ formatDisplayValue(draftExact.unitPrice) }} · 总价
                {{ formatDisplayValue(draftExact.quantity * draftExact.unitPrice) }}
              </span>
            </p>

            <div class="form-actions">
              <button class="action-btn" type="submit" :disabled="loading || !selectedSellCandidate">
                发布挂单
              </button>
            </div>
          </form>

          <div v-if="!sellCandidates.length" class="card-empty">当前没有可上架的库存物品。</div>
        </article>

        <article class="market-card">
          <div class="card-head">
            <h3>我的挂单</h3>
            <span class="head-tag">{{ snapshot.myListings.length }} 条</span>
          </div>

          <div v-if="snapshot.myListings.length" class="listing-list">
            <div v-for="listing in snapshot.myListings" :key="listing.id" class="listing-row">
              <div class="listing-main">
                <div class="listing-title">
                  <strong>{{ listing.itemName }}</strong>
                  <span class="tag">剩余 {{ formatDisplayValue(listing.quantityAvailable) }}</span>
                </div>
                <div class="listing-meta">
                  <span>单价 {{ snapshot.currency.icon }} {{ formatDisplayValue(listing.unitPrice) }}</span>
                  <span>总价 {{ snapshot.currency.icon }} {{ formatDisplayValue(listing.totalPrice) }}</span>
                </div>
                <div class="listing-meta muted">
                  <span>上架 {{ formatDateTime(listing.listedAt) }}</span>
                  <span v-if="listing.expiresAt">⏰ {{ formatRemainingTime(listing.expiresAt) }}</span>
                </div>
              </div>
              <button
                class="action-btn secondary"
                type="button"
                :disabled="loading"
                @click="emit('cancel-listing', listing.id)"
              >
                撤销
              </button>
            </div>
          </div>

          <div v-else class="card-empty">你还没有正在出售的挂单。</div>
        </article>

        <article class="market-card market-card--wide">
          <div class="card-head">
            <h3>市场挂单</h3>
          </div>

          <div v-if="publicListings.length" class="listing-list">
            <div v-for="listing in publicListings" :key="listing.id" class="listing-row">
              <div class="listing-main">
                <div class="listing-title">
                  <strong>{{ listing.itemName }}</strong>
                  <div class="listing-tags">
                    <span class="tag">{{ listing.sellerName }}</span>
                    <span class="tag">剩余 {{ formatDisplayValue(listing.quantityAvailable) }}</span>
                  </div>
                </div>
                <div class="listing-meta">
                  <span>单价 {{ snapshot.currency.icon }} {{ formatDisplayValue(listing.unitPrice) }}</span>
                  <span>总价 {{ snapshot.currency.icon }} {{ formatDisplayValue(listing.totalPrice) }}</span>
                </div>
                <div class="listing-meta muted">
                  <span>上架 {{ formatDateTime(listing.listedAt) }}</span>
                  <span v-if="listing.expiresAt">⏰ {{ formatRemainingTime(listing.expiresAt) }}</span>
                </div>
              </div>
              <div class="listing-actions">
                <div class="field-input-group field-input-group--inside field-input-group--compact buy-quantity-group">
                  <input
                    v-model.trim="buyQuantities[listing.id]"
                    class="market-field market-field--compact market-field--with-btn"
                    type="text"
                    inputmode="decimal"
                    placeholder="1K"
                    :disabled="loading"
                    @input="sanitizeBuyQuantityInput(listing.id, listing.quantityAvailable)"
                    @blur="normalizeBuyQuantity(listing.id, listing.quantityAvailable)"
                  />
                  <button
                    class="inline-btn inline-btn--inside inline-btn--max"
                    type="button"
                    :disabled="loading || listing.quantityAvailable <= 0"
                    @click="fillBuyMaxQuantity(listing.id, listing.quantityAvailable)"
                  >
                    MAX
                  </button>
                </div>
                <button
                  class="action-btn listing-buy-btn"
                  type="button"
                  :disabled="loading || listing.quantityAvailable <= 0"
                  @click="
                    emit('buy', {
                      listingId: listing.id,
                      quantity: clampBuyQuantity(listing.id, listing.quantityAvailable),
                    })
                  "
                >
                  购买
                </button>
              </div>
            </div>
          </div>

          <div v-else class="card-empty">当前没有其他玩家可购买的挂单。</div>
        </article>

        <article class="market-card market-card--wide">
          <div class="card-head">
            <h3>成交记录</h3>
            <span class="head-tag">最近 {{ snapshot.tradeRecords.length }} 条</span>
          </div>

          <div v-if="snapshot.tradeRecords.length" class="record-list">
            <div v-for="record in snapshot.tradeRecords" :key="record.id" class="record-row">
              <span class="record-badge" :class="`record-badge--${record.type}`">
                {{ recordTag(record.type) }}
              </span>
              <div class="record-main">
                <p class="record-summary">
                  <strong class="record-item">{{ record.itemName }}</strong>
                  <span>|</span>
                  <span>
                    {{ snapshot.currency.icon }} {{ formatDisplayValue(record.unitPrice) }} ×
                    {{ formatDisplayValue(record.quantity) }} · {{ record.counterpart }}
                  </span>
                </p>
                <p class="record-time">{{ formatDateTime(record.createdAt) }}</p>
              </div>
              <div class="record-side">
                <strong>{{ snapshot.currency.icon }} {{ formatDisplayValue(record.totalPrice) }}</strong>
                <span class="status" :class="record.status">{{ statusText(record.status) }}</span>
              </div>
            </div>
          </div>

          <div v-else class="card-empty">还没有成交记录。</div>
        </article>
      </section>
    </template>

    <div v-else class="empty">点击左侧“市场”按钮后加载市场快照。</div>
  </div>
</template>

<script setup lang="ts">
import { computed, reactive, watch } from 'vue'
import type { MarketSellCandidate, MarketSnapshot } from '@/types/Market'

const props = defineProps<{
  snapshot: MarketSnapshot | null
  loading: boolean
  error: string
  sellCandidates: MarketSellCandidate[]
}>()

const emit = defineEmits<{
  refresh: []
  buy: [{ listingId: number; quantity: number }]
  'create-listing': [{ itemId: string; quantity: number; unitPrice: number }]
  'cancel-listing': [listingId: number]
}>()

const draft = reactive({
  itemId: '',
  quantity: '1',
  unitPrice: '12',
})

const draftExact = reactive({
  quantity: 1,
  unitPrice: 12,
})

const buyQuantities = reactive<Record<number, string>>({})
const buyQuantityExact = reactive<Record<number, number>>({})

const selectedSellCandidate = computed(
  () => props.sellCandidates.find((item) => item.itemId === draft.itemId) ?? null,
)

const publicListings = computed(
  () => props.snapshot?.listings.filter((listing) => !listing.sellerIsSelf) ?? [],
)

const formatCurrency = (value: number) => {
  const safe = Number.isFinite(value) ? value : 0
  return new Intl.NumberFormat('zh-CN', { maximumFractionDigits: 0 }).format(safe)
}

const formatDisplayValue = (value: number) => formatCompactValue(value)

const formatCompactValue = (value: number) => {
  const safe = Number.isFinite(value) ? Math.max(0, value) : 0
  if (safe < 1000) return formatCurrency(safe)
  return new Intl.NumberFormat('en-US', {
    notation: 'compact',
    compactDisplay: 'short',
    maximumFractionDigits: safe >= 100000 ? 0 : 1,
  }).format(safe)
}

const parseCompactNumber = (value: string) => {
  const normalized = String(value || '')
    .trim()
    .replace(/,/g, '')
    .replace(/\s+/g, '')
    .toUpperCase()
  if (!normalized) return 1
  const match = normalized.match(/^(\d+(?:\.\d+)?)([KMB])?$/)
  if (!match) return 1
  const base = Number(match[1])
  if (!Number.isFinite(base) || base <= 0) return 1
  const multiplierMap: Record<string, number> = { K: 1_000, M: 1_000_000, B: 1_000_000_000 }
  const multiplier = multiplierMap[match[2] || ''] ?? 1
  return Math.max(1, Math.floor(base * multiplier))
}

const sanitizeCompactInput = (value: string) => {
  const normalized = String(value ?? '')
    .toUpperCase()
    .replace(/[^0-9.KMB]/g, '')
  let numberPart = ''
  let hasDot = false
  let suffix = ''
  for (const ch of normalized) {
    if (/\d/.test(ch)) {
      if (!suffix) numberPart += ch
      continue
    }
    if (ch === '.') {
      if (!suffix && !hasDot) {
        numberPart = numberPart ? `${numberPart}.` : '0.'
        hasDot = true
      }
      continue
    }
    if (!suffix && /[KMB]/.test(ch) && numberPart && numberPart !== '0.') {
      suffix = ch
    }
  }
  return `${numberPart}${suffix}`
}

const sanitizeDraftInput = (field: 'quantity' | 'unitPrice') => {
  const sanitized = sanitizeCompactInput(draft[field])
  draft[field] = sanitized
  draftExact[field] = parseCompactNumber(sanitized)
}

const sanitizeDraftQuantityInput = () => {
  sanitizeDraftInput('quantity')
}

const normalizeDraftInput = (field: 'quantity' | 'unitPrice') => {
  let safe = draftExact[field] || parseCompactNumber(draft[field])
  if (field === 'quantity') {
    const max = Math.max(1, selectedSellCandidate.value?.quantity || 1)
    safe = Math.max(1, Math.min(max, safe))
  } else {
    safe = Math.max(1, safe)
  }
  draftExact[field] = safe
  draft[field] = formatCompactValue(safe)
}

const fillDraftMaxQuantity = () => {
  if (!selectedSellCandidate.value) return
  const safe = Math.max(1, selectedSellCandidate.value.quantity)
  draftExact.quantity = safe
  draft.quantity = formatCompactValue(safe)
}

const normalizeBuyQuantity = (listingId: number, max: number) => {
  const safe = Math.max(
    1,
    Math.min(Math.max(1, max), buyQuantityExact[listingId] ?? parseCompactNumber(buyQuantities[listingId] ?? '')),
  )
  buyQuantityExact[listingId] = safe
  buyQuantities[listingId] = formatCompactValue(safe)
}

const sanitizeBuyQuantityInput = (listingId: number, _max: number) => {
  const sanitized = sanitizeCompactInput(buyQuantities[listingId] ?? '')
  buyQuantities[listingId] = sanitized
  buyQuantityExact[listingId] = parseCompactNumber(sanitized)
}

const formatDateTime = (value: string) =>
  new Intl.DateTimeFormat('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(value))

const formatRemainingTime = (value: string) => {
  const diff = new Date(value).getTime() - Date.now()
  if (!Number.isFinite(diff) || diff <= 0) return '已到期'
  const totalMinutes = Math.ceil(diff / 60000)
  const days = Math.floor(totalMinutes / 1440)
  const hours = Math.floor((totalMinutes % 1440) / 60)
  const minutes = totalMinutes % 60
  if (days > 0) return `${days}天${hours}小时`
  if (hours > 0) return `${hours}小时${minutes}分钟`
  return `${Math.max(1, minutes)}分钟`
}

const statusText = (status: 'settled') => ({ settled: '已结算' })[status]

const recordTag = (type: 'buy' | 'sell' | 'market') =>
  ({ buy: 'BUY', sell: 'SELL', market: 'MKT' })[type]

const clampBuyQuantity = (listingId: number, max: number) => {
  const value = buyQuantityExact[listingId] ?? parseCompactNumber(buyQuantities[listingId] ?? '')
  const safe = Math.max(1, Math.min(Math.max(1, max), value))
  buyQuantityExact[listingId] = safe
  buyQuantities[listingId] = formatCompactValue(safe)
  return safe
}

const fillBuyMaxQuantity = (listingId: number, max: number) => {
  const safe = Math.max(1, Math.floor(Number(max) || 1))
  buyQuantityExact[listingId] = safe
  buyQuantities[listingId] = formatCompactValue(safe)
}

const submitCreate = () => {
  if (!selectedSellCandidate.value) return
  const quantity = Math.max(
    1,
    Math.min(
      Math.max(1, selectedSellCandidate.value.quantity),
      draftExact.quantity || parseCompactNumber(draft.quantity),
    ),
  )
  const unitPrice = Math.max(1, draftExact.unitPrice || parseCompactNumber(draft.unitPrice))
  draftExact.quantity = quantity
  draftExact.unitPrice = unitPrice
  draft.quantity = formatCompactValue(quantity)
  draft.unitPrice = formatCompactValue(unitPrice)
  emit('create-listing', {
    itemId: selectedSellCandidate.value.itemId,
    quantity,
    unitPrice,
  })
}

watch(
  () => props.sellCandidates,
  (next) => {
    if (!next.length) {
      draft.itemId = ''
      draft.quantity = '1'
      draftExact.quantity = 1
      return
    }
    if (!next.some((item) => item.itemId === draft.itemId)) {
      draft.itemId = next[0]?.itemId || ''
    }
    const max = Math.max(1, selectedSellCandidate.value?.quantity || 1)
    const safe = Math.max(1, Math.min(max, draftExact.quantity || parseCompactNumber(draft.quantity)))
    draftExact.quantity = safe
    draft.quantity = formatCompactValue(safe)
  },
  { immediate: true, deep: true },
)

watch(
  () => props.snapshot?.listings,
  (next) => {
    for (const listing of next ?? []) {
      const current = buyQuantityExact[listing.id] ?? parseCompactNumber(buyQuantities[listing.id] ?? '')
      const safe = Math.max(1, Math.min(listing.quantityAvailable, current))
      buyQuantityExact[listing.id] = safe
      buyQuantities[listing.id] = formatCompactValue(safe)
    }
  },
  { immediate: true, deep: true },
)
</script>

<style scoped>
.market-panel {
  width: 100%;
  min-width: 0;
  min-height: 0;
  overflow: auto;
  display: grid;
  align-content: start;
  gap: 12px;
}

.market-hero,
.market-summary,
.market-card {
  border: 1px solid var(--border);
  border-radius: 14px;
  background: color-mix(in srgb, var(--surface-2) 82%, transparent);
}

.market-hero {
  padding: 14px;
  display: flex;
  justify-content: space-between;
  gap: 16px;
  align-items: center;
}

.market-hero h2,
.market-card h3 {
  margin: 0;
}

.market-subtitle {
  margin: 6px 0 0;
  color: var(--muted);
  font-size: 0.9rem;
}

.market-refresh,
.action-btn {
  min-height: 38px;
  border-radius: 999px;
  border: 1px solid rgba(255, 255, 255, 0.12);
  padding: 0 14px;
  color: rgba(255, 255, 255, 0.96);
  font-weight: 800;
  cursor: pointer;
  background: linear-gradient(
    135deg,
    color-mix(in srgb, var(--brand) 82%, #000),
    color-mix(in srgb, var(--brand-2) 78%, #000)
  );
}

.market-refresh:disabled,
.action-btn:disabled {
  cursor: not-allowed;
  opacity: 0.7;
}

.action-btn.secondary {
  background: color-mix(in srgb, var(--surface-2) 86%, transparent);
  color: var(--text);
  border-color: var(--border);
}

.market-summary {
  padding: 12px;
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 10px;
}

.summary-card {
  border: 1px solid var(--border);
  border-radius: 12px;
  padding: 10px;
  background: color-mix(in srgb, var(--surface) 90%, transparent);
  display: grid;
  gap: 4px;
}

.summary-label {
  font-size: 0.78rem;
  color: var(--muted-2);
}

.market-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(280px, 0.9fr);
  gap: 12px;
  align-items: start;
}

.market-card {
  padding: 12px;
  display: grid;
  gap: 10px;
  min-width: 0;
}

.market-card--wide {
  grid-column: 1 / -1;
}

.card-head {
  display: flex;
  justify-content: space-between;
  gap: 10px;
  align-items: center;
}

.head-tag,
.tag,
.status {
  border-radius: 999px;
  padding: 2px 8px;
  font-size: 0.74rem;
  white-space: nowrap;
}

.head-tag,
.tag {
  border: 1px solid var(--border);
  background: color-mix(in srgb, var(--surface) 90%, transparent);
}

.sell-form,
.listing-list,
.record-list {
  display: grid;
  gap: 10px;
}

.listing-row,
.record-row {
  border: 1px solid var(--border);
  border-radius: 12px;
  padding: 10px;
  background: color-mix(in srgb, var(--surface) 90%, transparent);
}

.listing-row,
.record-row {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  align-items: flex-start;
}

.listing-main,
.record-main,
.record-side,
.field {
  display: grid;
  gap: 6px;
}

.listing-title {
  display: flex;
  gap: 10px;
  align-items: center;
  flex-wrap: wrap;
}

.listing-tags,
.form-actions {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
}

.field-row {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 148px));
  gap: 16px;
  justify-content: start;
}

.field-row .field {
  min-width: 0;
}

.field--half {
  width: 100%;
  max-width: 148px;
}

.listing-meta,
.record-row p,
.record-side span,
.form-hint {
  margin: 0;
  color: var(--muted);
  font-size: 0.86rem;
}

.form-hint {
  display: grid;
  gap: 2px;
}

.form-hint-row {
  display: block;
}

.listing-meta {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
}

.listing-meta.muted {
  color: var(--muted-2);
}

.record-side {
  flex: 0 0 120px;
  width: 120px;
  justify-items: end;
  text-align: right;
}

.record-summary {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  align-items: baseline;
}

.record-main {
  flex: 1 1 auto;
  min-width: 0;
}

.record-badge {
  flex: 0 0 auto;
  align-self: flex-start;
  border: 0;
  border-radius: 999px;
  padding: 4px 10px;
  font-size: 0.72rem;
  font-weight: 700;
  letter-spacing: 0.08em;
  line-height: 1;
  color: var(--text);
}

.record-badge--buy {
  background: color-mix(in srgb, #2563eb 24%, var(--surface-2));
}

.record-badge--sell {
  background: color-mix(in srgb, #ea580c 24%, var(--surface-2));
}

.record-badge--market {
  background: color-mix(in srgb, #7c3aed 22%, var(--surface-2));
}

.record-item {
  color: var(--text);
  font-size: inherit;
}

.record-time {
  color: var(--muted-2);
  font-size: 0.8rem;
  opacity: 0.84;
}

.field span {
  font-size: 0.8rem;
  color: var(--muted);
}

.field-input-group {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
  width: 100%;
}

.field-input-group--inside {
  position: relative;
  display: block;
  width: 100%;
}

.field-input-group--compact {
  gap: 6px;
}

.market-field {
  box-sizing: border-box;
  width: 100%;
  min-height: 34px;
  border-radius: 10px;
  border: 1px solid var(--border);
  background: color-mix(in srgb, var(--surface) 92%, transparent);
  color: var(--text);
  padding: 0 10px;
}

.market-field--with-btn {
  padding-right: 46px;
}

.market-field--compact {
  width: 118px;
}

.buy-quantity-group {
  width: 118px;
  flex: 0 0 118px;
}

.buy-quantity-group .market-field--compact {
  width: 100%;
}

.market-field[type='number'] {
  appearance: textfield;
  -moz-appearance: textfield;
}

.market-field[type='number']::-webkit-outer-spin-button,
.market-field[type='number']::-webkit-inner-spin-button {
  -webkit-appearance: none;
  margin: 0;
}

.listing-actions {
  display: grid;
  gap: 8px;
  justify-items: end;
  align-content: start;
}

.listing-buy-btn {
  min-width: 118px;
}

.inline-btn {
  flex: 0 0 auto;
  min-height: 24px;
  padding: 0 9px;
  border: 0;
  border-radius: 999px;
  background: color-mix(in srgb, var(--brand) 28%, var(--surface-2));
  color: color-mix(in srgb, var(--text) 96%, var(--brand));
  font: inherit;
  font-size: 0.7rem;
  font-weight: 600;
  letter-spacing: 0.02em;
  cursor: pointer;
  transition:
    background 0.2s ease,
    color 0.2s ease,
    opacity 0.2s ease,
    transform 0.2s ease;
}

.inline-btn--inside {
  position: absolute;
  right: 5px;
  top: 50%;
  z-index: 1;
  transform: translateY(-50%);
}

.inline-btn--max {
  background: color-mix(in srgb, var(--brand) 40%, var(--surface-2));
  color: color-mix(in srgb, var(--text) 96%, var(--brand));
}

.inline-btn:hover:not(:disabled) {
  transform: translateY(calc(-50% - 1px));
}

.inline-btn--max:hover:not(:disabled) {
  background: color-mix(in srgb, var(--brand) 52%, var(--surface-2));
}

.inline-btn:disabled {
  opacity: 0.56;
  cursor: not-allowed;
}

@media (max-width: 640px) {
  .field-row {
    grid-template-columns: minmax(0, 1fr);
  }

  .field--half {
    max-width: none;
  }
}

.card-empty {
  border: 1px dashed var(--border);
  border-radius: 10px;
  padding: 12px;
  color: var(--muted);
}

.status.settled {
  color: var(--success);
}

.error {
  color: var(--danger);
  font-weight: 800;
  margin: 0;
}

.empty {
  color: var(--muted);
  border: 1px dashed var(--border);
  border-radius: 10px;
  padding: 14px;
}

@media (max-width: 1280px) {
  .market-summary,
  .market-grid {
    grid-template-columns: 1fr;
  }
}
</style>

