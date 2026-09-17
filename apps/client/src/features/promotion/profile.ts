export type PromoterStatus = 'NOT_APPLIED' | 'PENDING' | 'REJECTED' | 'ENABLED' | 'DISABLED'
export interface PromoterProfile {
  status: PromoterStatus
  reason: string
  applicationId: string
  capabilities: { canApply: boolean; canPromote: boolean; canReadHistory: boolean }
}
export type PromotionViewState =
  | { kind: 'login-required' }
  | { kind: 'loading' }
  | { kind: 'error' }
  | { kind: 'profile'; profile: PromoterProfile }

const permissions: Record<PromoterStatus, [boolean, boolean, boolean]> = {
  NOT_APPLIED: [true, false, false], PENDING: [false, false, false],
  REJECTED: [true, false, false], ENABLED: [false, true, true], DISABLED: [false, false, true],
}

function record(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

export function parsePromoterProfile(response: unknown): PromoterProfile | null {
  if (!record(response) || response.code !== 0 || !record(response.data)) return null
  const data = response.data
  if (typeof data.status !== 'string' || !Object.prototype.hasOwnProperty.call(permissions, data.status)) return null
  if (typeof data.reason !== 'string' || typeof data.applicationId !== 'string' || !record(data.capabilities)) return null
  const status = data.status as PromoterStatus
  const [canApply, canPromote, canReadHistory] = permissions[status]
  if (data.capabilities.canApply !== canApply || data.capabilities.canPromote !== canPromote || data.capabilities.canReadHistory !== canReadHistory) return null
  return { status, reason: data.reason, applicationId: data.applicationId, capabilities: { canApply, canPromote, canReadHistory } }
}
