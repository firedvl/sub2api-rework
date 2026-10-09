import { describe, expect, it } from 'vitest'
import { currencySymbol, formatPaymentAmount, paymentCurrencyFractionDigits } from '../currency'

describe('formatPaymentAmount', () => {
  it('uses backend payment precision even when Intl currency defaults differ', () => {
    expect(paymentCurrencyFractionDigits('IQD')).toBe(3)
    expect(formatPaymentAmount(85.85, 'IQD', 'en-US')).toContain('85.850')
    expect(paymentCurrencyFractionDigits('JPY')).toBe(0)
    expect(paymentCurrencyFractionDigits('USD')).toBe(2)
  })
  it('uses the currency default fraction digits', () => {
    expect(formatPaymentAmount(100, 'JPY', 'en-US')).not.toContain('.00')
    expect(formatPaymentAmount(100, 'KRW', 'en-US')).not.toContain('.00')
    expect(formatPaymentAmount(100, 'HKD', 'en-US')).toContain('.00')
  })
})

describe('currencySymbol', () => {
  it('maps common payment currencies and falls back safely', () => {
    expect(currencySymbol('USD')).toBe('$')
    expect(currencySymbol('cny')).toBe('¥')
    expect(currencySymbol('EUR')).toBe('€')
    expect(currencySymbol('')).toBe('¥')
    expect(currencySymbol('XYZ')).toBe('XYZ')
  })
})
