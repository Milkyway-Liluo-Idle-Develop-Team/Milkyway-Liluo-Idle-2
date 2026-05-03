export interface MarketCurrency {
  code: string
  name: string
  icon: string
  precision: number
}

export interface MarketListing {
  id: number
  sellerUid: number | null
  sellerName: string
  itemId: string
  itemName: string
  quantityTotal: number
  quantityAvailable: number
  unitPrice: number
  totalPrice: number
  currencyCode: string
  status: 'active' | 'sold_out' | 'cancelled' | 'expired'
  listedAt: string
  expiresAt: string | null
  sellerIsSelf: boolean
}

export interface MarketTradeRecord {
  id: number
  listingId: number
  sellerUid: number | null
  buyerUid: number | null
  sellerName: string
  buyerName: string
  itemId: string
  itemName: string
  quantity: number
  unitPrice: number
  totalPrice: number
  currencyCode: string
  status: 'settled'
  createdAt: string
  type: 'buy' | 'sell' | 'market'
  counterpart: string
  counterpartUid: number | null
}

export interface MarketSnapshot {
  currency: MarketCurrency
  walletBalance: number
  activeListings: number
  totalVolume24h: number
  lastUpdatedAt: string
  listings: MarketListing[]
  myListings: MarketListing[]
  tradeRecords: MarketTradeRecord[]
}

export interface MarketSellCandidate {
  itemId: string
  itemName: string
  quantity: number
  classification: string
}
