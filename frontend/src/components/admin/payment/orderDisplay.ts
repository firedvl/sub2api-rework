/**
 * Display helpers shared by the admin order table, order drawer, and dashboard.
 * Class strings stay literal so Tailwind keeps the component classes.
 */
import type { CurrencyAmounts } from '@/types/payment'

const STATUS_BADGE: Record<string, string> = {
  PENDING: 'badge badge-warning',
  PAID: 'badge badge-primary',
  RECHARGING: 'badge badge-primary',
  COMPLETED: 'badge badge-success',
  EXPIRED: 'badge badge-gray',
  CANCELLED: 'badge badge-gray',
  FAILED: 'badge badge-danger',
  REFUND_REQUESTED: 'badge badge-warning',
  REFUNDING: 'badge badge-warning',
  REFUND_PENDING: 'badge badge-warning',
  PARTIALLY_REFUNDED: 'badge badge-purple',
  REFUNDED: 'badge badge-purple',
  REFUND_FAILED: 'badge badge-danger',
}

export function orderStatusBadgeClass(status: string): string {
  return STATUS_BADGE[status] || 'badge badge-gray'
}

export function sortedAmounts(amounts: CurrencyAmounts | null | undefined): [string, number][] {
  return Object.entries(amounts || {}).sort(([left], [right]) => left.localeCompare(right))
}

export function formatMoney(currency: string, amount: number): string {
  try {
    return new Intl.NumberFormat(undefined, { style: 'currency', currency }).format(amount)
  } catch {
    return `${amount.toFixed(2)} ${currency}`
  }
}

export type OrderActionKind = 'cancel' | 'retry' | 'refund' | 'queryRefund'

export interface OrderAction {
  kind: OrderActionKind
  icon: 'x' | 'refresh' | 'check' | 'dollar'
  labelKey: string
  tone: 'default' | 'danger'
}

/** The one status-specific action an order offers (same rules as before the redesign). */
export function orderAction(status: string): OrderAction | null {
  switch (status) {
    case 'PENDING':
      return { kind: 'cancel', icon: 'x', labelKey: 'payment.orders.cancel', tone: 'default' }
    case 'FAILED':
      return { kind: 'retry', icon: 'refresh', labelKey: 'payment.admin.retry', tone: 'default' }
    case 'REFUND_REQUESTED':
      return { kind: 'refund', icon: 'check', labelKey: 'payment.admin.approveRefund', tone: 'default' }
    case 'REFUND_FAILED':
      return { kind: 'refund', icon: 'refresh', labelKey: 'payment.admin.retryRefund', tone: 'default' }
    case 'REFUND_PENDING':
      return { kind: 'queryRefund', icon: 'refresh', labelKey: 'payment.admin.queryRefundStatus', tone: 'default' }
    case 'COMPLETED':
    case 'PARTIALLY_REFUNDED':
      return { kind: 'refund', icon: 'dollar', labelKey: 'payment.admin.refund', tone: 'danger' }
    default:
      return null
  }
}
