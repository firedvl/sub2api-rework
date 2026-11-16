import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { adminAPI } from '@/api/admin'

const errorAccounts = ref(0)
const limitedAccounts = ref(0)
const totalAccounts = ref(0)
const loaded = ref(false)
let subscribers = 0
let timer: number | undefined

async function refresh() {
  if (document.visibilityState === 'hidden') return
  try {
    const stats = await adminAPI.dashboard.getStats()
    errorAccounts.value = Number(stats.error_accounts) || 0
    limitedAccounts.value = (Number(stats.ratelimit_accounts) || 0) + (Number(stats.overload_accounts) || 0)
    totalAccounts.value = Number(stats.total_accounts) || 0
    loaded.value = true
  } catch {
    // The shell keeps its last known state; pages report their own load errors.
  }
}

/**
 * A light, shared heartbeat for the shell: account fault counts from the
 * dashboard summary, refreshed every minute while any shell component uses it.
 */
export function useGatewayPulse(enabled: () => boolean) {
  onMounted(() => {
    if (!enabled()) return
    subscribers += 1
    if (subscribers === 1) {
      void refresh()
      timer = window.setInterval(refresh, 60_000)
    }
  })

  onBeforeUnmount(() => {
    if (!enabled() || subscribers === 0) return
    subscribers -= 1
    if (subscribers === 0 && timer) {
      window.clearInterval(timer)
      timer = undefined
    }
  })

  const attention = computed(() => errorAccounts.value + limitedAccounts.value)
  const tone = computed<'ok' | 'warn' | 'bad'>(() => {
    if (errorAccounts.value > 0 && errorAccounts.value >= totalAccounts.value) return 'bad'
    return attention.value > 0 ? 'warn' : 'ok'
  })

  return { attention, errorAccounts, limitedAccounts, totalAccounts, tone, loaded, refresh }
}
