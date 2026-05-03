export type MarketCurrency = {
  code: string
  name: string
  icon: string
  precision: number
}

export type MarketListing = {
  id: string
  itemId: string
  itemName: string
  sellerName: string
  unitPrice: number
  quantity: number
  totalPrice: number
  listedAt: string
  expiresAt: string
  tags: string[]
}

export type MarketTradeRecord = {
  id: string
  type: 'buy' | 'sell'
  itemName: string
  quantity: number
  unitPrice: number
  totalPrice: number
  counterpart: string
  createdAt: string
  status: 'settled' | 'delivered' | 'cancelled'
}

export type MarketMail = {
  id: string
  title: string
  kind: 'sale' | 'purchase' | 'system'
  content: string
  attachments: Array<{
    type: 'currency' | 'item'
    label: string
    amount: number
  }>
  createdAt: string
  read: boolean
}

export type MarketSnapshot = {
  currency: MarketCurrency
  walletBalance: number
  activeListings: number
  totalVolume24h: number
  lastUpdatedAt: string
  listings: MarketListing[]
  tradeRecords: MarketTradeRecord[]
  mails: MarketMail[]
}

const currency: MarketCurrency = {
  code: 'coin',
  name: '金币',
  icon: '◎',
  precision: 0,
}

const baseListings: MarketListing[] = [
  {
    id: 'listing-wood-1',
    itemId: 'wood_log',
    itemName: '原木',
    sellerName: 'Aster',
    unitPrice: 14,
    quantity: 180,
    totalPrice: 2520,
    listedAt: '2026-04-25T11:18:00+08:00',
    expiresAt: '2026-04-26T11:18:00+08:00',
    tags: ['材料', '低价'],
  },
  {
    id: 'listing-ore-1',
    itemId: 'copper_ore',
    itemName: '铜矿石',
    sellerName: 'Nemu',
    unitPrice: 33,
    quantity: 72,
    totalPrice: 2376,
    listedAt: '2026-04-25T12:40:00+08:00',
    expiresAt: '2026-04-26T12:40:00+08:00',
    tags: ['矿石'],
  },
  {
    id: 'listing-sword-1',
    itemId: 'wood_sword',
    itemName: '木剑',
    sellerName: 'Kanon',
    unitPrice: 260,
    quantity: 3,
    totalPrice: 780,
    listedAt: '2026-04-25T13:55:00+08:00',
    expiresAt: '2026-04-26T13:55:00+08:00',
    tags: ['装备', '稀有'],
  },
  {
    id: 'listing-herb-1',
    itemId: 'fresh_herb',
    itemName: '新鲜草药',
    sellerName: 'Moca',
    unitPrice: 19,
    quantity: 95,
    totalPrice: 1805,
    listedAt: '2026-04-25T14:12:00+08:00',
    expiresAt: '2026-04-26T14:12:00+08:00',
    tags: ['消耗品'],
  },
]

const baseTradeRecords: MarketTradeRecord[] = [
  {
    id: 'trade-1',
    type: 'sell',
    itemName: '石块',
    quantity: 40,
    unitPrice: 8,
    totalPrice: 320,
    counterpart: 'Shiro',
    createdAt: '2026-04-25T09:24:00+08:00',
    status: 'settled',
  },
  {
    id: 'trade-2',
    type: 'buy',
    itemName: '粗制箭矢',
    quantity: 120,
    unitPrice: 5,
    totalPrice: 600,
    counterpart: 'Hina',
    createdAt: '2026-04-25T10:02:00+08:00',
    status: 'delivered',
  },
  {
    id: 'trade-3',
    type: 'sell',
    itemName: '铜锭',
    quantity: 12,
    unitPrice: 74,
    totalPrice: 888,
    counterpart: 'Sayo',
    createdAt: '2026-04-25T10:48:00+08:00',
    status: 'settled',
  },
]

const baseMails: MarketMail[] = [
  {
    id: 'mail-1',
    title: '市场售出成功',
    kind: 'sale',
    content: '你上架的“石块 x40”已售出，收益已发放。',
    attachments: [{ type: 'currency', label: '金币', amount: 320 }],
    createdAt: '2026-04-25T09:25:00+08:00',
    read: false,
  },
  {
    id: 'mail-2',
    title: '购买物品送达',
    kind: 'purchase',
    content: '你购买的“粗制箭矢 x120”已送达邮箱附件。',
    attachments: [{ type: 'item', label: '粗制箭矢', amount: 120 }],
    createdAt: '2026-04-25T10:03:00+08:00',
    read: true,
  },
]

const delay = (ms: number) => new Promise<void>((resolve) => window.setTimeout(resolve, ms))

export async function fetchMockMarketSnapshot(): Promise<MarketSnapshot> {
  await delay(480)

  const refreshedListings = baseListings.map((listing, index) => {
    const quantityDelta = index % 2 === 0 ? 0 : (Date.now() + index) % 5
    const quantity = Math.max(1, listing.quantity - quantityDelta)
    return {
      ...listing,
      quantity,
      totalPrice: quantity * listing.unitPrice,
    }
  })

  return {
    currency,
    walletBalance: 12840,
    activeListings: refreshedListings.length,
    totalVolume24h: 38640,
    lastUpdatedAt: new Date().toISOString(),
    listings: refreshedListings,
    tradeRecords: baseTradeRecords,
    mails: baseMails,
  }
}
