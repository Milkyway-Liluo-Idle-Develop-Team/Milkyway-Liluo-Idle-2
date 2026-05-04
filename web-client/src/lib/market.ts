import { getJson, postJson } from '@/lib/api'
import type { MarketSnapshot } from '@/types/Market'

type MarketEnvelope<T> = {
  success: true
  data: T
}

async function unwrapMarketRequest<T>(
  request: Promise<{ ok: true; data: MarketEnvelope<T> } | { ok: false; status: number; error: string }>,
): Promise<T> {
  const res = await request
  if (!res.ok) {
    throw new Error(res.error || '市场请求失败')
  }
  return res.data.data
}

export async function fetchMarketSnapshot() {
  return unwrapMarketRequest<MarketSnapshot>(
    getJson<MarketEnvelope<MarketSnapshot>>('/api/market', {
      credentials: 'include',
    }),
  )
}

export async function createMarketListing(payload: {
  itemId: string
  quantity: number
  unitPrice: number
}) {
  return unwrapMarketRequest<unknown>(
    postJson<MarketEnvelope<unknown>>(
      '/api/market/listings',
      {
        item_id: payload.itemId,
        quantity: payload.quantity,
        unit_price: payload.unitPrice,
      },
      { credentials: 'include' },
    ),
  )
}

export async function buyMarketListing(payload: { listingId: number; quantity: number }) {
  return unwrapMarketRequest<unknown>(
    postJson<MarketEnvelope<unknown>>(
      '/api/market/buy',
      {
        listing_id: payload.listingId,
        quantity: payload.quantity,
      },
      { credentials: 'include' },
    ),
  )
}

export async function cancelMarketListing(listingId: number) {
  return unwrapMarketRequest<unknown>(
    postJson<MarketEnvelope<unknown>>(
      `/api/market/listings/${listingId}/cancel`,
      {},
      { credentials: 'include' },
    ),
  )
}
