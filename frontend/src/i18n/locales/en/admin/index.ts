import overview from './overview'
import stats from './stats'
import channels from './channels'
import accounts from './accounts'
import resources from './resources'
import ops from './ops'
import settings from './settings'
import audit from './audit'
import promptAudit from './promptAudit'
import plugins from './plugins'

export default {
  ...overview,
  ...stats,
  ...channels,
  ...accounts,
  ...resources,
  ...ops,
  ...settings,
  ...audit,
  ...promptAudit,
  ...plugins,
  operatorAssistant: {
    title: 'Ask Gateway',
    ready: 'Ready',
    clear: 'Clear conversation',
    model: 'Model',
    auto: 'Auto',
    noModels: 'No routable text models are currently available.',
    modelsError: 'Model options could not be loaded.',
    you: 'You',
    thinking: 'Checking current gateway state...',
    stopped: 'Stopped',
    contextRefreshed: 'Context refreshed just now',
    placeholder: 'Ask about capacity, accounts, errors, models, or release state',
    stop: 'Stop response',
    send: 'Send message',
    emptyResponse: 'The model returned an empty response.',
    requestError: 'Ask Gateway could not complete the request.',
    retry: 'Retry',
    prompts: {
      attention: 'What needs attention?',
      capacity: 'Why is capacity low?',
      errors: 'Summarize recent errors',
      models: 'Which models are healthy?',
    },
  },
}
