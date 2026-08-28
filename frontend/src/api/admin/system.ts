import { apiClient } from '../client'

export type ReleaseState =
  | 'up_to_date'
  | 'upstream_available'
  | 'compatibility_pending'
  | 'rework_build_available'
  | 'update_ready'
  | 'update_blocked'
  | 'update_failed'

export type Compatibility = 'pending' | 'approved' | 'blocked'
export type UpdaterState =
  | 'unavailable'
  | 'idle'
  | 'preparing'
  | 'prepared'
  | 'installing'
  | 'rolling_back'
  | 'succeeded'
  | 'failed'
  | 'critical'

export interface ReleaseNotes {
  upstream: string
  rework: string
  compatibility: string
  migrations: string
  rollback: string
}

export interface OperationSummary {
  operation_id: string
  action: 'prepare' | 'install' | 'rollback'
  actor: string
  source_version: string
  target_version: string
  started_at: string
  finished_at?: string
  result: string
  rollback_result?: string
  error?: string
}

export interface UpdaterStatus {
  schema_version: number
  updater_version: string
  healthy: boolean
  state: UpdaterState
  busy: boolean
  installed_version: string
  prepared_version?: string
  rollback_version?: string
  current_migration: number
  last_attempt?: OperationSummary
  last_rollback?: OperationSummary
  last_error?: string
  updated_at: string
}

export interface UpdateInfo {
  schema_version: number
  current_version: string
  current_git_commit: string
  build_date: string
  build_type: string
  update_channel: string
  update_policy: string
  upstream_baseline: string
  upstream_baseline_sha: string
  latest_upstream: string
  latest_upstream_url?: string
  latest_rework_version?: string
  latest_compatible_rework?: string
  state: ReleaseState
  installable: boolean
  compatibility?: Compatibility
  release_date?: string
  migration_min?: number
  migration_max?: number
  minimum_updater_version?: string
  release_notes?: ReleaseNotes
  checked_at: string
  cached: boolean
  warning?: string
}

export interface UpdateStatus extends UpdateInfo {
  updater: UpdaterStatus
}

export interface VersionInfo {
  version: string
  git_commit: string
  build_date: string
  build_type: string
  upstream_baseline: string
  upstream_baseline_sha: string
  update_channel: string
  update_policy: string
}

export interface OperationAccepted {
  operation_id: string
  action: 'prepare' | 'install' | 'rollback'
  state: string
}

export async function getVersion(): Promise<VersionInfo> {
  const { data } = await apiClient.get<VersionInfo>('/admin/system/version')
  return data
}

export async function checkUpdates(force = false): Promise<UpdateStatus> {
  const { data } = await apiClient.get<UpdateStatus>('/admin/system/check-updates', {
    params: force ? { force: 'true' } : undefined
  })
  return data
}

export async function prepareUpdate(version: string): Promise<OperationAccepted> {
  const { data } = await apiClient.post<OperationAccepted>('/admin/system/prepare', { version })
  return data
}

export async function installUpdate(version: string, confirmation: string): Promise<OperationAccepted> {
  const { data } = await apiClient.post<OperationAccepted>('/admin/system/install', { version, confirmation })
  return data
}

export async function rollbackUpdate(version: string, confirmation: string): Promise<OperationAccepted> {
  const { data } = await apiClient.post<OperationAccepted>('/admin/system/rollback', { version, confirmation })
  return data
}

export const systemAPI = { getVersion, checkUpdates, prepareUpdate, installUpdate, rollbackUpdate }

export default systemAPI
