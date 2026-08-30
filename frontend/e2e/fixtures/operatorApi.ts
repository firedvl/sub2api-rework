import type { Page, Route } from '@playwright/test'
import {
  getOperatorFixtureData,
  getOperatorImportPreview,
  getOperatorImportResult,
  isOperatorFixtureReadRequest,
  operatorFixtureUser,
  operatorAssistantFixturePrompt,
  operatorAssistantFixtureSSE,
  operatorAssistantFixtureMetadata,
  OPERATOR_FIXTURE_ACCOUNTS_ETAG,
  OPERATOR_FIXTURE_TOKEN,
  type RunMode,
  type SessionRole,
} from './operatorData.ts'

export type { RunMode, SessionRole } from './operatorData.ts'

async function fulfill(
  route: Route,
  data: unknown,
  options: { status?: number; headers?: Record<string, string> } = {},
): Promise<void> {
  await route.fulfill({
    status: options.status ?? 200,
    headers: options.headers,
    contentType: 'application/json',
    body: JSON.stringify({ code: 0, message: 'ok', data }),
  })
}

export async function seedSession(
  page: Page,
  role: SessionRole = 'admin',
  runMode: RunMode = 'standard',
): Promise<void> {
  await page.addInitScript(({ user, token }) => {
    localStorage.setItem('auth_token', token)
    localStorage.setItem('auth_user', JSON.stringify(user))
  }, { user: operatorFixtureUser(role, runMode), token: OPERATOR_FIXTURE_TOKEN })
}

export async function installOperatorApiMock(
  page: Page,
  role: SessionRole = 'admin',
  runMode: RunMode = 'standard',
): Promise<void> {
  const assistantAttempts = new Map<string, number>()
  await page.route('**/setup/status*', (route) =>
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ data: { needs_setup: false, step: 'complete' } }),
    }),
  )

  await page.route('**/api/v1/**', async (route) => {
    const request = route.request()
    const pathname = new URL(request.url()).pathname
    const method = request.method().toUpperCase()

    if (method === 'POST' && pathname === '/api/v1/admin/operator-assistant') {
      if (role !== 'admin') return fulfill(route, {}, { status: 403 })
      const body = request.postDataJSON()
      const prompt = operatorAssistantFixturePrompt(body)
      const attempt = (assistantAttempts.get(prompt) || 0) + 1
      assistantAttempts.set(prompt, attempt)
      if (prompt.toLowerCase().includes('retry fixture') && attempt === 1) {
        return fulfill(route, {}, { status: 503 })
      }
      if (prompt.toLowerCase().includes('slow fixture')) {
        await new Promise((resolve) => setTimeout(resolve, 1_000))
      }
      const metadata = operatorAssistantFixtureMetadata(body)
      return route.fulfill({
        status: 200,
        contentType: 'text/event-stream',
        headers: {
          'Cache-Control': 'no-store',
          'X-Gateway-Model': metadata.model,
          'X-Gateway-Model-Selection': metadata.selection,
          'X-Gateway-Provider': metadata.provider,
        },
        body: operatorAssistantFixtureSSE(body),
      })
    }

    if (method === 'POST' && pathname === '/api/v1/admin/accounts/data/preview') {
      return fulfill(route, getOperatorImportPreview(request.postDataJSON()))
    }
    if (method === 'POST' && pathname === '/api/v1/admin/accounts/data') {
      return fulfill(route, getOperatorImportResult(request.postDataJSON()))
    }

    if (!isOperatorFixtureReadRequest(method, pathname)) {
      return fulfill(route, { fixture_review: true, read_only: true }, { status: 405 })
    }

    if (pathname === '/api/v1/admin/accounts') {
      const headers = { etag: OPERATOR_FIXTURE_ACCOUNTS_ETAG }
      if (request.headers()['if-none-match'] === OPERATOR_FIXTURE_ACCOUNTS_ETAG) {
        return route.fulfill({ status: 304, headers })
      }
      return fulfill(route, getOperatorFixtureData(pathname, role, runMode), { headers })
    }

    return fulfill(route, getOperatorFixtureData(pathname, role, runMode))
  })
}
