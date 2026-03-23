declare global {
  interface Window {
    WF: typeof import('./global/index').WF
  }
}

export {}
