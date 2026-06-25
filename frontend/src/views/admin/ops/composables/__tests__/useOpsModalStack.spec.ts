import { describe, expect, it } from 'vitest'
import { useOpsModalStack } from '../useOpsModalStack'

describe('useOpsModalStack', () => {
  it('keeps request details open when a child error detail closes', () => {
    const modals = useOpsModalStack()

    modals.openRequestDetails({ title: 'Slow requests', kind: 'error' }, 'Requests')
    expect(modals.showRequestDetails.value).toBe(true)
    expect(modals.showErrorDetails.value).toBe(false)

    modals.openErrorDetail(101, 'request')
    expect(modals.showErrorModal.value).toBe(true)
    expect(modals.selectedErrorId.value).toBe(101)
    expect(modals.selectedErrorType.value).toBe('request')

    modals.showErrorModal.value = false
    expect(modals.showErrorModal.value).toBe(false)
    expect(modals.showRequestDetails.value).toBe(true)
    expect(modals.showErrorDetails.value).toBe(false)
  })

  it('resolves upstream child details from the active upstream parent layer', () => {
    const modals = useOpsModalStack()

    modals.openErrorDetails('upstream')
    expect(modals.showErrorDetails.value).toBe(true)
    expect(modals.errorDetailsType.value).toBe('upstream')

    modals.openErrorDetail(202)
    expect(modals.showErrorModal.value).toBe(true)
    expect(modals.selectedErrorId.value).toBe(202)
    expect(modals.selectedErrorType.value).toBe('upstream')

    modals.showErrorModal.value = false
    expect(modals.showErrorDetails.value).toBe(true)
    expect(modals.errorDetailsType.value).toBe('upstream')

    modals.showErrorDetails.value = false
    expect(modals.showErrorDetails.value).toBe(false)
    expect(modals.showErrorModal.value).toBe(false)
    expect(modals.selectedErrorId.value).toBeNull()
  })

  it('clears child detail state when switching parent detail layers', () => {
    const modals = useOpsModalStack()

    modals.openErrorDetails('request')
    modals.openErrorDetail(303, 'request')
    expect(modals.showErrorDetails.value).toBe(true)
    expect(modals.showErrorModal.value).toBe(true)

    modals.openRequestDetails(undefined, 'Requests')
    expect(modals.showRequestDetails.value).toBe(true)
    expect(modals.showErrorDetails.value).toBe(false)
    expect(modals.showErrorModal.value).toBe(false)
    expect(modals.selectedErrorId.value).toBeNull()
    expect(modals.requestDetailsPreset.value.title).toBe('Requests')
  })

  it('supports route-driven error details opening through the writable visible flag', () => {
    const modals = useOpsModalStack()

    modals.errorDetailsType.value = 'upstream'
    modals.showErrorDetails.value = true

    expect(modals.showErrorDetails.value).toBe(true)
    expect(modals.activeDetailsLayer.value).toBe('upstream-errors')
  })

  it('keeps error details preset while the parent error layer is open and clears it on close', () => {
    const modals = useOpsModalStack()

    modals.openErrorDetails('upstream', {
      title: 'Non-rate upstream',
      view: 'errors',
      phase: 'upstream',
      owner: 'provider',
      statusCode: 'non_rate_overload'
    })

    expect(modals.showErrorDetails.value).toBe(true)
    expect(modals.errorDetailsType.value).toBe('upstream')
    expect(modals.errorDetailsPreset.value?.statusCode).toBe('non_rate_overload')

    modals.showErrorDetails.value = false

    expect(modals.showErrorDetails.value).toBe(false)
    expect(modals.errorDetailsPreset.value).toBeNull()
  })
})
