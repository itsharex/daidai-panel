import request from './request'

export interface AndroidRuntimePreset {
  name: string
  label: string
  arch: string
  url: string
  strip_components: number
  check_bin: string
  size_mb: number
  note?: string
}

export interface AndroidRuntimeItem {
  name: string
  installed: boolean
  path: string
  version: string
}

export interface AndroidRuntimeStatus {
  supported: boolean
  arch: string
  bin_dir: string
  termux_detected: boolean
  runtimes: AndroidRuntimeItem[]
  presets: AndroidRuntimePreset[]
}

export const androidRuntimeApi = {
  status() {
    return request.get('/android-runtime/status') as Promise<{ data: AndroidRuntimeStatus }>
  },
  uninstall(name: string) {
    return request.post('/android-runtime/uninstall', { name }) as Promise<{ message: string }>
  },
}
