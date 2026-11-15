/**
 * Chart color for a provider. Color follows the provider, never its rank, so
 * a provider keeps one hue on every page. The four validated series slots go
 * to the main providers; any other platform uses the neutral series color.
 */
const SERIES: Record<string, string> = {
  gemini: 'var(--operator-series-1)',
  anthropic: 'var(--operator-series-2)',
  openai: 'var(--operator-series-3)',
  antigravity: 'var(--operator-series-4)',
}

const NAMES: Record<string, string> = {
  openai: 'OpenAI',
  anthropic: 'Anthropic',
  gemini: 'Gemini',
  antigravity: 'Antigravity',
  grok: 'Grok',
  kimi: 'Kimi',
  zhipu: 'Zhipu',
  deepseek: 'DeepSeek',
  minimax: 'MiniMax',
  opencode_go: 'OpenCode',
  typesafe: 'Typesafe',
  composite: 'Composite',
}

export const providerColor = (platform: string): string => SERIES[platform] ?? 'var(--operator-series-muted)'

export const providerName = (platform: string): string => NAMES[platform] ?? platform

/** Best-effort provider for a model name, used only to pick a logo. */
export function providerOfModel(model: string): string {
  const name = model.toLowerCase()
  if (name.startsWith('claude')) return 'anthropic'
  if (name.startsWith('gemini')) return 'gemini'
  if (/^(gpt|o\d|codex|chatgpt)/.test(name)) return 'openai'
  if (name.startsWith('grok')) return 'grok'
  if (name.startsWith('deepseek')) return 'deepseek'
  if (name.startsWith('kimi') || name.startsWith('moonshot')) return 'kimi'
  if (name.startsWith('glm')) return 'zhipu'
  if (name.startsWith('minimax') || name.startsWith('abab')) return 'minimax'
  return ''
}
