import { computed, ref } from 'vue'
import type { OpsRequestDetailsParams } from '@/api/admin/ops'

export type OpsErrorDetailType = 'request' | 'upstream'
export type OpsDetailsLayer = 'request-details' | 'request-errors' | 'upstream-errors'
export type OpsErrorDetailsView = 'errors' | 'excluded' | 'all'
export type OpsErrorDetailsStatusCode = number | 'other' | 'rate_overload' | 'non_rate_overload' | null

export interface OpsRequestDetailsPreset {
  title: string
  kind?: OpsRequestDetailsParams['kind']
  sort?: OpsRequestDetailsParams['sort']
  min_duration_ms?: number
  max_duration_ms?: number
}

export interface OpsErrorDetailsPreset {
  title?: string
  view?: OpsErrorDetailsView
  phase?: string
  owner?: string
  statusCode?: OpsErrorDetailsStatusCode
  startTime?: string
  endTime?: string
  eventScope?: string
  customerVisible?: boolean
  failureDomain?: string
  failureCategory?: string
  failureReason?: string
  resolutionOwner?: string
  poolOwnership?: string
  slaImpact?: boolean | 'unknown'
}

interface OpsErrorDetailLayer {
  show: boolean
  id: number | null
  type: OpsErrorDetailType
}

function detailsLayerForErrorType(type: OpsErrorDetailType): OpsDetailsLayer {
  return type === 'upstream' ? 'upstream-errors' : 'request-errors'
}

function errorTypeForDetailsLayer(layer: OpsDetailsLayer | null): OpsErrorDetailType {
  return layer === 'upstream-errors' ? 'upstream' : 'request'
}

function createDefaultRequestPreset(): OpsRequestDetailsPreset {
  return {
    title: '',
    kind: 'all',
    sort: 'created_at_desc'
  }
}

function createClosedErrorDetail(type: OpsErrorDetailType): OpsErrorDetailLayer {
  return {
    show: false,
    id: null,
    type
  }
}

export function useOpsModalStack() {
  const activeDetailsLayer = ref<OpsDetailsLayer | null>(null)
  const errorDetailsType = ref<OpsErrorDetailType>('request')
  const errorDetailsPreset = ref<OpsErrorDetailsPreset | null>(null)
  const requestDetailsPreset = ref<OpsRequestDetailsPreset>(createDefaultRequestPreset())
  const errorDetailLayer = ref<OpsErrorDetailLayer>(createClosedErrorDetail('request'))

  const selectedErrorId = computed(() => errorDetailLayer.value.id)
  const selectedErrorType = computed(() => errorDetailLayer.value.type)

  function closeErrorDetail() {
    errorDetailLayer.value = createClosedErrorDetail(errorDetailsType.value)
  }

  function closeActiveDetailsLayer(layer?: OpsDetailsLayer) {
    if (layer && activeDetailsLayer.value !== layer) return
    if (activeDetailsLayer.value === 'request-errors' || activeDetailsLayer.value === 'upstream-errors') {
      errorDetailsPreset.value = null
    }
    activeDetailsLayer.value = null
    closeErrorDetail()
  }

  function activateDetailsLayer(layer: OpsDetailsLayer) {
    activeDetailsLayer.value = layer
    if (layer !== 'request-details') {
      errorDetailsType.value = errorTypeForDetailsLayer(layer)
    }
  }

  const showRequestDetails = computed({
    get: () => activeDetailsLayer.value === 'request-details',
    set: (show) => {
      if (show) {
        activateDetailsLayer('request-details')
      } else {
        closeActiveDetailsLayer('request-details')
      }
    }
  })

  const showErrorDetails = computed({
    get: () => activeDetailsLayer.value === 'request-errors' || activeDetailsLayer.value === 'upstream-errors',
    set: (show) => {
      if (show) {
        activateDetailsLayer(detailsLayerForErrorType(errorDetailsType.value))
      } else if (activeDetailsLayer.value === 'request-errors' || activeDetailsLayer.value === 'upstream-errors') {
        closeActiveDetailsLayer(activeDetailsLayer.value)
      }
    }
  })

  const showErrorModal = computed({
    get: () => errorDetailLayer.value.show,
    set: (show) => {
      if (show) {
        // Opening must go through openErrorDetail so id and type stay consistent.
        if (!errorDetailLayer.value.id) return
        errorDetailLayer.value = { ...errorDetailLayer.value, show: true }
      } else {
        closeErrorDetail()
      }
    }
  })

  function openRequestDetails(preset: OpsRequestDetailsPreset | undefined, fallbackTitle: string) {
    const nextPreset = {
      ...createDefaultRequestPreset(),
      ...(preset ?? {})
    }
    if (!nextPreset.title) nextPreset.title = fallbackTitle

    requestDetailsPreset.value = nextPreset
    errorDetailsPreset.value = null
    closeErrorDetail()
    activateDetailsLayer('request-details')
  }

  function openErrorDetails(type: OpsErrorDetailType, preset?: OpsErrorDetailsPreset) {
    errorDetailsType.value = type
    errorDetailsPreset.value = preset ?? null
    closeErrorDetail()
    activateDetailsLayer(detailsLayerForErrorType(type))
  }

  function openErrorDetail(errorId: number, type?: OpsErrorDetailType) {
    const resolvedType = type ?? errorTypeForDetailsLayer(activeDetailsLayer.value)
    // Keep the parent error-list type aligned with direct child-detail launches.
    errorDetailsType.value = resolvedType
    errorDetailLayer.value = {
      show: true,
      id: errorId,
      type: resolvedType
    }
  }

  return {
    activeDetailsLayer,
    errorDetailsType,
    errorDetailsPreset,
    requestDetailsPreset,
    selectedErrorId,
    selectedErrorType,
    showErrorDetails,
    showErrorModal,
    showRequestDetails,
    openRequestDetails,
    openErrorDetails,
    openErrorDetail,
    closeErrorDetail,
    closeActiveDetailsLayer
  }
}
