import users from './users'
import subscriptions from './subscriptions'
import announcements from './announcements'
import proxies from './proxies'
import redeem from './redeem'
import promoCodes from './promoCodes'
import plugins from './plugins'
import riskControl from './riskControl'
import backup from './backup'
import orders from './orders'
import affiliates from './affiliates'
import promptAudit from './promptAudit'

// 重新设计的次级管理页与共享表格的简短文案。
export default {
  adminUi: {
    filters: '筛选',
    moreActions: '更多操作',
    details: '详情',
    clearFilters: '清除筛选',
    noMatches: '无匹配结果',
    table: {
      range: '{from}–{to} / 共 {total}',
      empty: '暂无数据',
      pageSize: '每页',
      sortBy: '按{column}排序',
    },
    users,
    subscriptions,
    announcements,
    proxies,
    redeem,
    promoCodes,
    plugins,
    riskControl,
    backup,
    orders,
    affiliates,
    promptAudit,
  },
}
