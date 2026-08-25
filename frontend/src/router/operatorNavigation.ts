export type OperatorAreaId = 'overview' | 'accounts' | 'models-routing' | 'activity' | 'settings'
export type OperatorRouteGate = 'channel-monitor' | 'ops-monitoring' | 'risk-control'

export interface OperatorSection {
  path: string
  labelKey: string
  gate?: OperatorRouteGate
  hideInSimpleMode?: boolean
}

export interface OperatorArea {
  id: OperatorAreaId
  labelKey: string
  primaryPath: string
  hideInSimpleMode?: boolean
  sections: OperatorSection[]
}

export const operatorAreas: OperatorArea[] = [
  {
    id: 'overview',
    labelKey: 'nav.overview',
    primaryPath: '/admin/dashboard',
    sections: [{ path: '/admin/dashboard', labelKey: 'nav.overview' }],
  },
  {
    id: 'accounts',
    labelKey: 'nav.accounts',
    primaryPath: '/admin/accounts',
    sections: [
      { path: '/admin/accounts', labelKey: 'nav.accounts' },
      { path: '/admin/proxies', labelKey: 'nav.proxies' },
    ],
  },
  {
    id: 'models-routing',
    labelKey: 'nav.modelsRouting',
    primaryPath: '/admin/groups',
    hideInSimpleMode: true,
    sections: [
      { path: '/admin/groups', labelKey: 'nav.groups' },
      { path: '/admin/channels/pricing', labelKey: 'nav.channelPricing' },
      {
        path: '/admin/channels/monitor',
        labelKey: 'nav.channelMonitor',
        gate: 'channel-monitor',
      },
    ],
  },
  {
    id: 'activity',
    labelKey: 'nav.activity',
    primaryPath: '/admin/ops',
    sections: [
      { path: '/admin/ops', labelKey: 'nav.ops', gate: 'ops-monitoring' },
      { path: '/admin/usage', labelKey: 'nav.usage' },
      { path: '/admin/audit-logs', labelKey: 'nav.auditLogs', hideInSimpleMode: true },
    ],
  },
  {
    id: 'settings',
    labelKey: 'nav.settings',
    primaryPath: '/admin/settings',
    sections: [
      { path: '/admin/settings', labelKey: 'nav.settings' },
      {
        path: '/admin/risk-control',
        labelKey: 'nav.contentModeration',
        gate: 'risk-control',
      },
      {
        path: '/admin/prompt-audit',
        labelKey: 'nav.promptAudit',
        gate: 'risk-control',
      },
    ],
  },
]

export function matchesOperatorPath(currentPath: string, sectionPath: string): boolean {
  return currentPath === sectionPath || currentPath.startsWith(`${sectionPath}/`)
}

export function getOperatorArea(currentPath: string): OperatorArea | undefined {
  return operatorAreas.find((area) =>
    area.sections.some((section) => matchesOperatorPath(currentPath, section.path)),
  )
}
