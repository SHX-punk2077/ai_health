import { http } from '@/utils/request'
import type { LoginResult, UserInfo } from '@/types/api'

export interface LoginPayload {
  account: string
  password: string
}

export interface RegisterPayload {
  username: string
  password: string
  nickname?: string
  email?: string
  phone?: string
}

export function login(payload: LoginPayload) {
  return http<LoginResult>({ url: '/auth/login', method: 'post', data: payload })
}

export function register(payload: RegisterPayload) {
  return http<UserInfo>({ url: '/auth/register', method: 'post', data: payload })
}

export function logout() {
  return http<null>({ url: '/auth/logout', method: 'post' })
}

export function profile() {
  return http<UserInfo>({ url: '/auth/profile', method: 'get' })
}

export function refreshToken(token: string) {
  return http<LoginResult>({ url: '/auth/refresh', method: 'post', data: { refreshToken: token } })
}
