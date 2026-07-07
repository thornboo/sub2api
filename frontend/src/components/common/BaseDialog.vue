<template>
  <Teleport to="body">
    <Transition name="modal">
      <div
        v-if="show"
        class="modal-overlay"
        :style="zIndexStyle"
        :aria-labelledby="dialogId"
        role="dialog"
        aria-modal="true"
        @click.self="handleClose"
      >
        <!-- Modal panel -->
        <div ref="dialogRef" :class="['modal-content', widthClasses]" @click.stop>
          <!-- Header -->
          <div class="modal-header">
            <h3 :id="dialogId" class="modal-title">
              {{ title }}
            </h3>
            <button
              v-if="showCloseButton"
              @click="handleCloseButton"
              class="-mr-2 rounded-xl p-2 text-stone-400 transition-colors hover:bg-stone-100 hover:text-stone-600 focus:outline-none focus-visible:ring-2 focus-visible:ring-emerald-500/30 focus-visible:ring-offset-2 focus-visible:ring-offset-white dark:text-stone-500 dark:hover:bg-white/[0.06] dark:hover:text-stone-300 dark:focus-visible:ring-offset-black"
              aria-label="Close modal"
            >
              <Icon name="x" size="md" />
            </button>
          </div>

          <!-- Body -->
          <div class="modal-body">
            <slot></slot>
          </div>

          <!-- Footer -->
          <div v-if="$slots.footer" class="modal-footer">
            <slot name="footer"></slot>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script lang="ts">
import { ref as createRef } from 'vue'

let dialogIdCounter = 0
let dialogInstanceIdCounter = 0
interface DialogStackEntry {
  id: number
  explicitZIndex?: number
}

const dialogStack = createRef<DialogStackEntry[]>([])

function getAutoDialogZIndex(index: number) {
  return 50 + Math.max(index, 0) * 10
}

// Interaction ownership follows the same effective z-index that users see.
// When z-index values tie, the later registered dialog wins, matching DOM paint order.
function getDialogZIndex(entry: DialogStackEntry, index: number) {
  return typeof entry.explicitZIndex === 'number'
    ? entry.explicitZIndex
    : getAutoDialogZIndex(index)
}

function getTopDialogId() {
  if (dialogStack.value.length === 0) return null

  let topEntry = dialogStack.value[0]
  let topZIndex = getDialogZIndex(topEntry, 0)
  let topStackIndex = 0

  dialogStack.value.forEach((entry, index) => {
    const zIndex = getDialogZIndex(entry, index)
    if (zIndex > topZIndex || (zIndex === topZIndex && index > topStackIndex)) {
      topEntry = entry
      topZIndex = zIndex
      topStackIndex = index
    }
  })

  return topEntry.id
}

function syncBodyScrollLock() {
  document.body.classList.toggle('modal-open', dialogStack.value.length > 0)
}

function registerDialog(id: number, explicitZIndex?: number) {
  const existingIndex = dialogStack.value.findIndex((entry) => entry.id === id)
  if (existingIndex >= 0) {
    updateDialogZIndex(id, explicitZIndex)
    return
  }
  dialogStack.value = [...dialogStack.value, { id, explicitZIndex }]
  syncBodyScrollLock()
}

function updateDialogZIndex(id: number, explicitZIndex?: number) {
  if (!dialogStack.value.some((entry) => entry.id === id)) return
  dialogStack.value = dialogStack.value.map((entry) => {
    if (entry.id !== id) return entry
    return { ...entry, explicitZIndex }
  })
}

function unregisterDialog(id: number) {
  if (!dialogStack.value.some((entry) => entry.id === id)) return
  dialogStack.value = dialogStack.value.filter((entry) => entry.id !== id)
  syncBodyScrollLock()
}
</script>

<script setup lang="ts">
import { computed, watch, onMounted, onUnmounted, ref, nextTick } from 'vue'
import Icon from '@/components/icons/Icon.vue'

// 生成唯一ID以避免多个对话框时ID冲突
const dialogId = `modal-title-${++dialogIdCounter}`

// 焦点管理
const dialogRef = ref<HTMLElement | null>(null)
let previousActiveElement: HTMLElement | null = null
const dialogInstanceId = ++dialogInstanceIdCounter

type DialogWidth = 'narrow' | 'normal' | 'wide' | 'extra-wide' | 'full'

interface Props {
  show: boolean
  title: string
  width?: DialogWidth
  closeOnEscape?: boolean
  closeOnClickOutside?: boolean
  showCloseButton?: boolean
  zIndex?: number
}

interface Emits {
  (e: 'close'): void
}

const props = withDefaults(defineProps<Props>(), {
  width: 'normal',
  closeOnEscape: true,
  closeOnClickOutside: false,
  showCloseButton: true
})

const emit = defineEmits<Emits>()

const stackIndex = computed(() => dialogStack.value.findIndex((entry) => entry.id === dialogInstanceId))
const isTopDialog = computed(() => {
  return getTopDialogId() === dialogInstanceId
})

// Custom z-index overrides the auto stack order when a caller needs an explicit layer.
const zIndexStyle = computed(() => {
  const entry = dialogStack.value[stackIndex.value]
  if (entry) return { zIndex: getDialogZIndex(entry, stackIndex.value) }
  if (typeof props.zIndex === 'number') return { zIndex: props.zIndex }
  return { zIndex: getAutoDialogZIndex(stackIndex.value) }
})

const widthClasses = computed(() => {
  // Width guidance: narrow=confirm/short prompts, normal=standard forms,
  // wide=multi-section forms or rich content, extra-wide=analytics/tables,
  // full=full-screen or very dense layouts.
  const widths: Record<DialogWidth, string> = {
    narrow: 'max-w-md',
    normal: 'max-w-lg',
    wide: 'w-full sm:max-w-2xl md:max-w-3xl lg:max-w-4xl',
    'extra-wide': 'w-full sm:max-w-3xl md:max-w-4xl lg:max-w-5xl xl:max-w-6xl',
    full: 'w-full sm:max-w-4xl md:max-w-5xl lg:max-w-6xl xl:max-w-7xl'
  }
  return widths[props.width]
})

const handleClose = () => {
  if (props.closeOnClickOutside && isTopDialog.value) {
    emit('close')
  }
}

const handleCloseButton = () => {
  if (isTopDialog.value) {
    emit('close')
  }
}

const handleEscape = (event: KeyboardEvent) => {
  if (props.show && props.closeOnEscape && isTopDialog.value && event.key === 'Escape') {
    emit('close')
  }
}

// Prevent body scroll when modal is open and manage focus
watch(
  () => props.show,
  async (isOpen) => {
    if (isOpen) {
      // 保存当前焦点元素
      previousActiveElement = document.activeElement as HTMLElement
      registerDialog(dialogInstanceId, props.zIndex)

      // 等待DOM更新后设置焦点到对话框
      await nextTick()
      if (dialogRef.value) {
        const firstFocusable = dialogRef.value.querySelector<HTMLElement>(
          'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'
        )
        firstFocusable?.focus()
      }
    } else {
      unregisterDialog(dialogInstanceId)
      // 恢复之前的焦点
      if (previousActiveElement && typeof previousActiveElement.focus === 'function') {
        previousActiveElement.focus()
      }
      previousActiveElement = null
    }
  },
  { immediate: true }
)

watch(
  () => props.zIndex,
  (zIndex) => {
    if (props.show) {
      updateDialogZIndex(dialogInstanceId, zIndex)
    }
  }
)

onMounted(() => {
  document.addEventListener('keydown', handleEscape)
})

onUnmounted(() => {
  document.removeEventListener('keydown', handleEscape)
  unregisterDialog(dialogInstanceId)
})
</script>
