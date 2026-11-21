import type { Token } from '@/types/api'

const TOKEN_KEY = 'aihealth_token'

export function getToken(): Token | null {
  const raw = localStorage.getItem(TOKEN_KEY)
  if (!raw) return null
  try {
    return JSON.parse(raw) as Token
  } catch {
    return null
  }
}

export function setToken(token: Token): void {
  localStorage.setItem(TOKEN_KEY, JSON.stringify(token))
}

export function clearToken(): void {
  localStorage.removeItem(TOKEN_KEY)
}

/** 未登录 / 令牌失效时统一跳转，避免 request 层依赖 router 实例 */
export function redirectToLogin(): void {
  const current = window.location.pathname + window.location.search
  if (current.startsWith('/login')) return
  window.location.assign('/login?redirect=' + encodeURIComponent(current))
}
