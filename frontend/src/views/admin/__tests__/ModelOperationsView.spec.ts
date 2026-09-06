import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import ModelOperationsView from '../ModelOperationsView.vue'

const { getAllIncludingInactive, getModelOperations } = vi.hoisted(() => ({
  getAllIncludingInactive: vi.fn(),
  getModelOperations: vi.fn(),
}))

vi.mock('@/api/admin/groups', () => ({
  getAllIncludingInactive,
  getModelOperations,
  default: { getAllIncludingInactive, getModelOperations },
}))
vi.mock('vue-i18n', async (importOriginal) => ({
  ...(await importOriginal<typeof import('vue-i18n')>()),
  useI18n: () => ({ t: (key: string) => key }),
}))

describe('ModelOperationsView', () => {
  beforeEach(() => {
    getAllIncludingInactive.mockReset()
    getModelOperations.mockReset()
    getAllIncludingInactive.mockResolvedValue([
      { id: 42, name: 'OpenAI', platform: 'openai', status: 'active' },
    ])
    getModelOperations.mockResolvedValue({
      generated_at: '2026-09-06T00:00:00Z',
      window: { start: '2026-08-30T00:00:00Z', end: '2026-09-06T00:00:00Z' },
      group: { id: 42, name: 'OpenAI', platform: 'openai' },
      models: [{
        model_id: 'gpt-6-astra', public_id: 'gpt-6-astra', actual_platform: 'openai',
        discovery_source: 'provider_discovery', configured: true, discovered: true,
        catalog_member: true, routable: false, current_availability: 'unavailable',
        rate_limited_or_cooldown: true, healthy: false, v1_models_visible: true,
        codex_picker_visible: true, route_type: 'direct', available_route_count: 0,
        recent_request_count: 3, recent_input_tokens: 10, recent_output_tokens: 2,
        recent_total_tokens: 12, sample_count: 3,
        latency_p50_ms: 120, latency_p95_ms: 480, ttft_p50_ms: 30, ttft_p95_ms: 90,
        output_tokens_per_second: 42.5, timing_sample_count: 5,
      }],
    })
  })

  it('separates catalog membership from current availability', async () => {
    const wrapper = mount(ModelOperationsView, {
      global: { stubs: { AppLayout: { template: '<div><slot /></div>' }, Icon: true } },
    })
    await flushPromises()

    expect(getModelOperations).toHaveBeenCalledWith(42)
    expect(wrapper.text()).toContain('gpt-6-astra')
    expect(wrapper.text()).toContain('admin.modelOperations.labels.catalogMember')
    expect(wrapper.text()).toContain('unavailable')
    expect(wrapper.text()).toContain('admin.modelOperations.labels.limited')
    expect(wrapper.text()).toContain('120 ms')
    expect(wrapper.text()).toContain('480 ms')
    expect(wrapper.text()).toContain('42.5 tok/s')
    expect(wrapper.text()).toContain('admin.modelOperations.labels.timingSamples')
    expect(wrapper.text()).not.toContain('grok')
  })

  it('renders unknown speed as N/A instead of zero', async () => {
    getModelOperations.mockResolvedValueOnce({
      generated_at: '2026-09-06T00:00:00Z',
      window: { start: '2026-08-30T00:00:00Z', end: '2026-09-06T00:00:00Z' },
      group: { id: 42, name: 'OpenAI', platform: 'openai' },
      models: [{
        model_id: 'gpt-sparse', public_id: 'gpt-sparse', actual_platform: 'openai',
        discovery_source: 'configured', configured: true, discovered: false,
        catalog_member: true, current_availability: 'unknown', v1_models_visible: true,
        codex_picker_visible: true, route_type: 'direct', recent_request_count: 2,
        recent_input_tokens: 10, recent_output_tokens: 2, recent_total_tokens: 12,
        sample_count: 2, latency_p50_ms: null, latency_p95_ms: null,
        ttft_p50_ms: null, ttft_p95_ms: null, output_tokens_per_second: null,
        timing_sample_count: 2,
      }],
    })
    const wrapper = mount(ModelOperationsView, {
      global: { stubs: { AppLayout: { template: '<div><slot /></div>' }, Icon: true } },
    })
    await flushPromises()

    expect(wrapper.text()).toContain('gpt-sparse')
    expect(wrapper.text()).toContain('admin.modelOperations.notAvailable')
    expect(wrapper.text()).not.toContain('0 ms')
    expect(wrapper.text()).not.toContain('0 tok/s')
  })
})
