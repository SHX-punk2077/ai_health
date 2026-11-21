import axios, {
  type AxiosInstance,
  type AxiosRequestConfig,
  type InternalAxiosRequestConfig,
} from 'axios'
import { ElMessage } from 'element-plus'
import { clearToken, getToken, redirectToLogin } from '@/utils/auth'
import type { ApiBody } from '@/types/api'

const instance: AxiosInstance = axios.create({
  baseURL: import.meta.env.VITE_API_BASE || '/api/v1',
  timeout: 30000,
})

instance.interceptors.request.use((config: InternalAxiosRequestConfig) => {
  const token = getToken()
  if (token?.accessToken) {
    config.headers.Authorization = 'Bearer ' + token.accessToken
  }
  return config
})

// 业务错误码统一提示；2xxxx 段属于认证类错误，直接踢回登录页
function handleBusinessError(code: number, message: string): void {
  if (code >= 20001 && code < 30000) {
    clearToken()
    redirectToLogin()
  }
  ElMessage.error(message || '请求失败')
}

instance.interceptors.response.use(
  (resp) => {
    const body = resp.data as ApiBody
    if (body && typeof body.code === 'number' && body.code !== 0) {
      handleBusinessError(body.code, body.message)
      return Promise.reject(new Error(body.message))
    }
    return resp
  },
  (error) => {
    const status = error?.response?.status
    const body = error?.response?.data as ApiBody | undefined
    if (body?.code) {
      handleBusinessError(body.code, body.message)
    } else if (status === 401) {
      clearToken()
      redirectToLogin()
      ElMessage.error('登录已过期，请重新登录')
    } else {
      ElMessage.error(error?.message || '网络异常')
    }
    return Promise.reject(error)
  },
)

/** 直接返回业务 data，调用方不必再解包 */
export async function http<T>(config: AxiosRequestConfig): Promise<T> {
  const resp = await instance.request<ApiBody<T>>(config)
  return resp.data.data
}

export default instance
