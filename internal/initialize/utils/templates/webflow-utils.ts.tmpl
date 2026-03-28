import { $, $$, onReady } from '@/utils/dom'

type FeatureMount<T extends Element = HTMLElement> = (element: T) => void
type PageMount<T extends Element = HTMLElement> = (pageRoot: T) => void

declare global {
  interface Window {
    Webflow?: Array<() => void>
  }
}

const mountedFeatures = new WeakMap<Element, Set<string>>()
const readyCallbacks = new WeakSet<() => void>()

function rememberMounted(element: Element, key: string) {
  const keys = mountedFeatures.get(element)
  if (keys) {
    if (keys.has(key)) {
      return false
    }

    keys.add(key)
    return true
  }

  mountedFeatures.set(element, new Set([key]))
  return true
}

export function onWebflowReady(callback: () => void) {
  if (typeof window === 'undefined') {
    return
  }

  if (readyCallbacks.has(callback)) {
    return
  }

  readyCallbacks.add(callback)

  let ran = false
  const runOnce = () => {
    if (ran) {
      return
    }

    ran = true
    callback()
  }

  window.Webflow ||= []
  window.Webflow.push(runOnce)

  // Fallback for local non-Webflow contexts like plain Vite preview.
  onReady(runOnce)
}

export function mountFeature<T extends Element = HTMLElement>(
  key: string,
  selector: string,
  mount: FeatureMount<T>,
  root: ParentNode = document
) {
  onWebflowReady(() => {
    for (const element of $$<T>(selector, root)) {
      if (!rememberMounted(element, key)) {
        continue
      }

      mount(element)
    }
  })
}

export function definePage<T extends Element = HTMLElement>(
  pageName: string,
  mount: PageMount<T>
) {
  onWebflowReady(() => {
    const pageRoot = $<T>(`[data-page="${pageName}"]`)
    if (!pageRoot) {
      return
    }

    pageRoot.setAttribute('data-page-ready', 'true')
    mount(pageRoot)
  })
}
