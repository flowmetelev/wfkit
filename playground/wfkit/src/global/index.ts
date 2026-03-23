import { $, onReady } from '@/utils/dom'
import { mountSiteGlobal } from './modules/site.global'

export const WF = {
  $,
  onReady,
  mountSiteGlobal
}

if (typeof window !== 'undefined') {
  window.WF = WF
}

mountSiteGlobal()
