import { defineConfig, loadEnv, Plugin } from 'vite'
import vue from '@vitejs/plugin-vue'
import checker from 'vite-plugin-checker'
import { resolve } from 'path'
import type { ServerResponse } from 'node:http'
import {
  getOperatorFixtureData,
  isOperatorFixtureReadRequest,
  operatorFixturePublicSettings,
  operatorFixtureUser,
  OPERATOR_FIXTURE_ACCOUNTS_ETAG,
  OPERATOR_FIXTURE_TOKEN,
} from './e2e/fixtures/operatorData'

function escapeHtml(value: string): string {
  return value.replace(/[&<>"']/g, (character) => ({
    '&': '&amp;',
    '<': '&lt;',
    '>': '&gt;',
    '"': '&quot;',
    "'": '&#39;',
  })[character] || character)
}

function isSafeImageUrl(value: string): boolean {
  const trimmed = value.trim()
  if ((trimmed.startsWith('/') && !trimmed.startsWith('//')) || /^data:image\//i.test(trimmed)) {
    return true
  }
  try {
    const parsed = new URL(trimmed)
    return parsed.protocol === 'http:' || parsed.protocol === 'https:'
  } catch {
    return false
  }
}

function injectBranding(html: string, config: { site_name?: string; site_logo?: string }): string {
  let brandedHtml = html
  const siteName = config.site_name?.trim()
  if (siteName) {
    brandedHtml = brandedHtml.replace(
      /<title>[^<]*<\/title>/i,
      `<title>${escapeHtml(siteName)} - AI API Gateway</title>`,
    )
  }

  const siteLogo = config.site_logo?.trim()
  if (siteLogo && isSafeImageUrl(siteLogo)) {
    brandedHtml = brandedHtml.replace(
      /<link\s+rel=["']icon["'][^>]*>/i,
      `<link rel="icon" href="${escapeHtml(siteLogo)}" />`,
    )
  }
  return brandedHtml
}

/**
 * Vite 插件：开发模式下注入公开配置到 index.html
 * 与生产模式的后端注入行为保持一致，消除闪烁
 */
function injectPublicSettings(backendUrl: string): Plugin {
  return {
    name: 'inject-public-settings',
    apply: 'serve',
    transformIndexHtml: {
      order: 'pre',
      async handler(html) {
        try {
          const response = await fetch(`${backendUrl}/api/v1/settings/public`, {
            signal: AbortSignal.timeout(2000)
          })
          if (response.ok) {
            const data = await response.json()
            if (data.code === 0 && data.data) {
              const script = `<script>window.__APP_CONFIG__=${JSON.stringify(data.data)};</script>`
              return injectBranding(html, data.data).replace('</head>', `${script}\n</head>`)
            }
          }
        } catch (e) {
          console.warn('[vite] 无法获取公开配置，将回退到 API 调用:', (e as Error).message)
        }
        return html
      }
    }
  }
}

function sendReviewJson(response: ServerResponse, status: number, payload: unknown): void {
  response.statusCode = status
  response.setHeader('Content-Type', 'application/json')
  response.setHeader('Cache-Control', 'no-store')
  response.setHeader('X-Sub2API-Fixture-Review', 'true')
  response.end(JSON.stringify(payload))
}

function operatorReviewFixtures(landingPath = '/admin/dashboard'): Plugin {
  const fixtureUser = operatorFixtureUser()
  const appConfig = JSON.stringify(operatorFixturePublicSettings)
  const sessionUser = JSON.stringify(fixtureUser)
  const reviewInitScript = `<script>(()=>{window.__OPERATOR_REVIEW_MODE__=true;window.__APP_CONFIG__=${appConfig};const keys=['auth_token','auth_user','refresh_token','token_expires_at'];localStorage.setItem('admin_guide_1_admin_v4_interactive','true');if(location.pathname==='/login'){keys.forEach((key)=>localStorage.removeItem(key));return;}localStorage.setItem('auth_token',${JSON.stringify(OPERATOR_FIXTURE_TOKEN)});localStorage.setItem('auth_user',${JSON.stringify(sessionUser)});})();</script>`

  return {
    name: 'operator-review-fixtures',
    apply: 'serve',
    transformIndexHtml(html) {
      return injectBranding(html, operatorFixturePublicSettings).replace(
        '</head>',
        `${reviewInitScript}\n</head>`,
      )
    },
    configureServer(server) {
      server.middlewares.use((request, response, next) => {
        const url = new URL(request.url || '/', 'http://operator-review.local')
        const pathname = url.pathname
        const method = (request.method || 'GET').toUpperCase()

        if (pathname === '/' && String(request.headers.accept || '').includes('text/html')) {
          response.statusCode = 302
          response.setHeader('Location', landingPath)
          response.end()
          return
        }

        if (pathname === '/setup/status') {
          sendReviewJson(response, 200, { data: { needs_setup: false, step: 'complete' } })
          return
        }

        if (!pathname.startsWith('/api/v1/')) {
          next()
          return
        }

        if (!isOperatorFixtureReadRequest(method, pathname)) {
          sendReviewJson(response, 405, {
            code: 405,
            message: 'Local fixture review is read-only',
            data: { fixture_review: true, read_only: true },
          })
          return
        }

        if (
          pathname === '/api/v1/admin/accounts' &&
          request.headers['if-none-match'] === OPERATOR_FIXTURE_ACCOUNTS_ETAG
        ) {
          response.statusCode = 304
          response.setHeader('ETag', OPERATOR_FIXTURE_ACCOUNTS_ETAG)
          response.setHeader('Cache-Control', 'no-store')
          response.setHeader('X-Sub2API-Fixture-Review', 'true')
          response.end()
          return
        }

        if (pathname === '/api/v1/admin/accounts') {
          response.setHeader('ETag', OPERATOR_FIXTURE_ACCOUNTS_ETAG)
        }
        sendReviewJson(response, 200, {
          code: 0,
          message: 'ok',
          data: getOperatorFixtureData(pathname),
        })
      })
    },
  }
}

function preventPrototypeBundleLeak(): Plugin {
  const markers = ['/ui-lab', 'operator-prototype-lab']

  return {
    name: 'prevent-operator-prototype-bundle-leak',
    apply: 'build',
    generateBundle(_options, bundle) {
      for (const output of Object.values(bundle)) {
        const source = output.type === 'chunk'
          ? output.code
          : typeof output.source === 'string' ? output.source : ''
        if (markers.some((marker) => source.includes(marker))) {
          this.error(`Production bundle contains operator prototype code in ${output.fileName}`)
        }
      }
    },
  }
}

export default defineConfig(({ mode }) => {
  // 加载环境变量
  const env = loadEnv(mode, process.cwd(), '')
  const isOperatorReview = mode === 'operator-review'
  const isPrototypeReview = mode === 'operator-prototypes'
  const isFixtureReview = isOperatorReview || isPrototypeReview
  const backendUrl = env.VITE_DEV_PROXY_TARGET || 'http://localhost:8080'
  const devPort = Number(env.VITE_DEV_PORT || 3000)

  return {
    plugins: [
      vue(),
      checker({
        vueTsc: true
      }),
      isFixtureReview
        ? operatorReviewFixtures(isPrototypeReview ? '/ui-lab' : '/admin/dashboard')
        : injectPublicSettings(backendUrl),
      ...(isPrototypeReview ? [] : [preventPrototypeBundleLeak()]),
    ],
  resolve: {
    alias: {
      '@': resolve(__dirname, 'src'),
      // 使用 vue-i18n 运行时版本，避免 CSP unsafe-eval 问题
      'vue-i18n': 'vue-i18n/dist/vue-i18n.runtime.esm-bundler.js'
    }
  },
  define: {
    // 启用 vue-i18n JIT 编译，在 CSP 环境下处理消息插值
    // JIT 编译器生成 AST 对象而非 JS 代码，无需 unsafe-eval
    __INTLIFY_JIT_COMPILATION__: true
  },
  build: {
    outDir: '../backend/internal/web/dist',
    emptyOutDir: true,
    rollupOptions: {
      output: {
        /**
         * 手动分包配置
         * 分离第三方库并按功能合并应用代码，避免循环依赖
         */
        manualChunks(id: string) {
          if (id.includes('node_modules')) {
            // Vue 核心库
            if (
              id.includes('/vue/') ||
              id.includes('/vue-router/') ||
              id.includes('/pinia/') ||
              id.includes('/@vue/')
            ) {
              return 'vendor-vue'
            }

            // UI 工具库（较大，单独分离）
            if (id.includes('/@vueuse/') || id.includes('/xlsx/')) {
              return 'vendor-ui'
            }

            // 图表库
            if (id.includes('/chart.js/') || id.includes('/vue-chartjs/')) {
              return 'vendor-chart'
            }

            // 国际化
            if (id.includes('/vue-i18n/') || id.includes('/@intlify/')) {
              return 'vendor-i18n'
            }

            // Stripe 仅在支付流程中按需加载，避免进入首页公共依赖。
            if (id.includes('/@stripe/stripe-js/')) {
              return 'vendor-stripe'
            }

            // 其他小型第三方库合并
            return 'vendor-misc'
          }

          // 应用代码：按入口点自动分包，不手动干预
          // 这样可以避免循环依赖，同时保持合理的 chunk 数量
        }
      }
    }
  },
    server: {
      host: isFixtureReview ? '127.0.0.1' : '0.0.0.0',
      port: isFixtureReview ? 4174 : devPort,
      strictPort: isFixtureReview,
      proxy: isFixtureReview ? undefined : {
        '/api': {
          target: backendUrl,
          changeOrigin: true
        },
        '/v1': {
          target: backendUrl,
          changeOrigin: true
        },
        '/setup': {
          target: backendUrl,
          changeOrigin: true
        }
      }
    }
  }
})
