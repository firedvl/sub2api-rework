export function normalizePlanType(value?: string | null): string {
  return (value || '').trim().toLowerCase().replace(/[\s_-]+/g, '')
}

export function openAIPlanTypeLabel(value?: string | null): string {
  switch (normalizePlanType(value)) {
    case 'plus': return 'Plus'
    case 'pro':
    case 'chatgptpro': return 'Pro 20x'
    case 'prolite': return 'Pro 5x'
    case 'selfservebusinessprolite': return 'Business Premium'
    case 'team': return 'Business Standard'
    case 'free': return 'Free'
    default: return ''
  }
}
