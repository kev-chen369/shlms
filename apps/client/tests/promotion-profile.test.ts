import { describe, expect, it } from 'vitest'
import { parsePromoterProfile } from '../src/features/promotion/profile'

const valid = {
  code: 0, message: 'success', data: {
    status: 'ENABLED', reason: '', applicationId: 'application-1',
    capabilities: { canApply: false, canPromote: true, canReadHistory: true },
  },
}

describe('promoter profile API boundary', () => {
  it('accepts the existing owner-only API response without requiring a client-supplied owner', () => {
    expect(parsePromoterProfile(valid)).toEqual({
      status: 'ENABLED', reason: '', applicationId: 'application-1',
      capabilities: { canApply: false, canPromote: true, canReadHistory: true },
    })
  })
  it.each([
    ['NOT_APPLIED', true, false, false], ['PENDING', false, false, false],
    ['REJECTED', true, false, false], ['DISABLED', false, false, true],
  ])('accepts the server permission contract for %s', (status, canApply, canPromote, canReadHistory) => {
    const profile = parsePromoterProfile({ code: 0, data: { status, reason: '', applicationId: '', capabilities: { canApply, canPromote, canReadHistory } } })
    expect(profile?.status).toBe(status)
  })
  it.each([
    null, [], {}, { ...valid, code: '0' }, { ...valid, code: 'UNAUTHORIZED' },
    { ...valid, data: { ...valid.data, status: 'READY' } },
    { ...valid, data: { ...valid.data, reason: null } },
    { ...valid, data: { ...valid.data, applicationId: 42 } },
    { ...valid, data: { ...valid.data, capabilities: { ...valid.data.capabilities, canPromote: 'true' } } },
    { ...valid, data: { ...valid.data, status: 'DISABLED' } },
  ])('fails closed for malformed or contradictory responses %#', response => {
    expect(parsePromoterProfile(response)).toBeNull()
  })
})
