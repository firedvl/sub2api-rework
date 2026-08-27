import type { Page, Route } from '@playwright/test'
import {
  getOperatorFixtureData,
  isOperatorFixtureReadRequest,
  operatorFixtureUser,
  OPERATOR_FIXTURE_ACCOUNTS_ETAG,
  OPERATOR_FIXTURE_TOKEN,
  type RunMode,
  type SessionRole,
} from './operatorData'

export type { RunMode, SessionRole } from './operatorData'

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
    const guide = user.role === 'admin' ? 'admin_guide' : 'user_guide'
    localStorage.setItem(`${guide}_${user.id}_${user.role}_v4_interactive`, 'true')
  }, { user: operatorFixtureUser(role, runMode), token: OPERATOR_FIXTURE_TOKEN })
}

export async function installOperatorApiMock(
  page: Page,
  role: SessionRole = 'admin',
  runMode: RunMode = 'standard',
): Promise<void> {
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
