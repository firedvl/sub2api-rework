import type { Account } from '@/types'

export function getActiveModelRateLimits(extra: Account['extra'], now = new Date()) {
  const nowTimestamp = now.getTime()

  return Object.entries(extra?.model_rate_limits ?? {}).filter(([, info]) => {
    const resetTimestamp = new Date(info.rate_limit_reset_at).getTime()
    return Number.isFinite(resetTimestamp) && resetTimestamp > nowTimestamp
  })
}
