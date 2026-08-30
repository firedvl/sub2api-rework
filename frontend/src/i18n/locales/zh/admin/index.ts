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
    title: '询问 Gateway',
    ready: '就绪',
    clear: '清空对话',
    model: '模型',
    auto: '自动',
    noModels: '当前没有可路由的文本模型。',
    modelsError: '无法加载模型选项。',
    you: '你',
    thinking: '正在检查当前网关状态...',
    stopped: '已停止',
    contextRefreshed: '上下文刚刚刷新',
    placeholder: '询问容量、账号、错误、模型或发布状态',
    stop: '停止回答',
    send: '发送消息',
    emptyResponse: '模型返回了空响应。',
    requestError: 'Ask Gateway 无法完成请求。',
    retry: '重试',
    prompts: {
      attention: '现在有什么需要关注？',
      capacity: '为什么容量较低？',
      errors: '总结最近的错误',
      models: '哪些模型处于健康状态？',
    },
  },
}
