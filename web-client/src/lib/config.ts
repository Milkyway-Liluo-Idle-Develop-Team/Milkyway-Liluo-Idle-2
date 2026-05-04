/** Global runtime switch: true = JSON text frames, false = protobuf binary. */
export const USE_JSON = (import.meta.env.VITE_WS_CODEC ?? 'json') === 'json'
