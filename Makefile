.PHONY: build build-backend build-frontend test test-backend test-frontend test-frontend-critical

FRONTEND_CRITICAL_VITEST := \
	src/i18n/__tests__/localeKeyCompleteness.spec.ts \
	src/api/__tests__/client.spec.ts \
	src/api/__tests__/tokenRefresh.spec.ts \
	src/api/__tests__/keys.bulkUpdate.spec.ts \
	src/components/keys/__tests__/BulkEditKeysModal.spec.ts \
	src/views/user/__tests__/KeysView.spec.ts \
	src/components/account/__tests__/OpenAIReferralCell.spec.ts \
	src/components/account/__tests__/OpenAIReferralCell.transport.spec.ts \
	src/components/account/__tests__/OpenAIQuotaResetCell.spark_shadow.spec.ts \
	src/api/__tests__/channelMonitorV2.spec.ts \
	src/views/auth/__tests__/LinuxDoCallbackView.spec.ts \
	src/views/auth/__tests__/WechatCallbackView.spec.ts \
	src/views/user/__tests__/PaymentView.spec.ts \
	src/views/user/__tests__/PaymentResultView.spec.ts \
	src/views/user/__tests__/ChannelStatusView.mode.spec.ts \
	src/components/user/profile/__tests__/ProfileInfoCard.spec.ts \
	src/components/admin/user/__tests__/UserPlatformQuotaModal.spec.ts \
	src/api/__tests__/admin.subscriptions.bulk.spec.ts \
	src/components/admin/subscription/__tests__/BulkSubscriptionActionDialog.spec.ts \
	src/components/admin/subscription/__tests__/bulkSubscriptionOperation.spec.ts \
	src/views/admin/__tests__/SubscriptionsView.bulkActions.spec.ts \
	src/views/admin/__tests__/SettingsView.spec.ts \
	src/views/admin/ops/components/__tests__/OpsRequestDetailsModal.spec.ts \
	src/views/admin/ops/components/__tests__/OpsErrorLogTable.spec.ts \
	src/views/admin/ops/components/__tests__/OpsOpenAITokenStatsCard.spec.ts \
	src/components/account/__tests__/EditAccountModal.grokMediaEligibility.spec.ts \
	src/api/__tests__/admin.proxies.spec.ts \
	src/components/common/__tests__/ProxySelector.testing.spec.ts \
	src/utils/__tests__/proxyExpiry.boundary.spec.ts \
	src/views/admin/__tests__/ProxiesView.filters.spec.ts \
	src/__tests__/integration/proxy-data-import.spec.ts \
	src/features/channel-monitor-v2/__tests__/designSystem.structure.spec.ts \
	src/features/channel-monitor-v2/__tests__/monitorFormat.spec.ts \
	src/features/channel-monitor-v2/__tests__/monitorZoom.spec.ts

# 一键编译前后端
build: build-backend build-frontend

# 编译后端（复用 backend/Makefile）
build-backend:
	@$(MAKE) -C backend build

# 编译前端（需要已安装依赖）
build-frontend:
	@pnpm --dir frontend run build

# 运行测试（后端 + 前端）
test: test-backend test-frontend

test-backend:
	@$(MAKE) -C backend test

test-frontend:
	@pnpm --dir frontend run lint:check
	@pnpm --dir frontend run typecheck
	@$(MAKE) test-frontend-critical

test-frontend-critical:
	@pnpm --dir frontend exec vitest run $(FRONTEND_CRITICAL_VITEST)
