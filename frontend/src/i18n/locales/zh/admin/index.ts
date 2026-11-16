import overview from './overview'
import stats from './stats'
import channels from './channels'
import accounts from './accounts'
import accountsUi from './accountsUi'
import resources from './resources'
import ops from './ops'
import settings from './settings'
import settingsUi from './settingsUi'
import audit from './audit'
import promptAudit from './promptAudit'
import plugins from './plugins'
import modelOperations from './modelOperations'
import viz from './viz'
import routingUi from './routingUi'
import adminUi from './adminUi'
import activityUi from './activityUi'

export default {
  ...overview,
  ...stats,
  ...channels,
  ...accounts,
  ...accountsUi,
  ...resources,
  ...ops,
  ...settings,
  ...settingsUi,
  ...audit,
  ...promptAudit,
  ...plugins,
  ...modelOperations,
  ...viz,
  ...routingUi,
  ...adminUi,
  ...activityUi,
}
