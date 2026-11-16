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

// Terse labels for the redesigned secondary admin pages and shared table chrome.
export default {
  adminUi: {
    filters: 'Filters',
    moreActions: 'More actions',
    details: 'Details',
    clearFilters: 'Clear filters',
    noMatches: 'No matches',
    table: {
      range: '{from}–{to} of {total}',
      empty: 'Nothing here yet',
      pageSize: 'Rows',
      sortBy: 'Sort by {column}',
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
