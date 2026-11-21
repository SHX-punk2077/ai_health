import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import * as authApi from '@/api/auth'
import { clearToken, getToken, setToken } from '@/utils/auth'
import type { UserInfo } from '@/types/api'

export const useUserStore = defineStore('user', () => {
  const user = ref<UserInfo | null>(null)
  const loading = ref(false)

  const isLogin = computed(() => !!getToken()?.accessToken)
  const roles = computed(() => user.value?.roles ?? [])

  // 权限判定：super_admin 后端直接返回通配符 *
  function hasPerm(perm: string): boolean {
    const perms = user.value?.permissions ?? []
    return perms.includes('*') || perms.includes(perm)
  }

  async function login(account: string, password: string): Promise<void> {
    loading.value = true
    try {
      const res = await authApi.login({ account, password })
      setToken({
        accessToken: res.accessToken,
        refreshToken: res.refreshToken,
        tokenType: res.tokenType,
        expiresIn: res.expiresIn,
        refreshExpiresIn: res.refreshExpiresIn,
      })
      user.value = res.user
    } finally {
      loading.value = false
    }
  }

  async function fetchProfile(): Promise<void> {
    user.value = await authApi.profile()
  }

  async function logout(): Promise<void> {
    try {
      await authApi.logout()
    } finally {
      clearToken()
      user.value = null
    }
  }

  return { user, loading, isLogin, roles, hasPerm, login, fetchProfile, logout }
})
