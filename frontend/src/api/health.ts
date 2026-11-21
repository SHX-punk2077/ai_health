import { http } from '@/utils/request'
import type { HealthProfile, HealthRecord, PageData } from '@/types/api'

export function getProfile() {
  return http<HealthProfile>({ url: '/health/profile', method: 'get' })
}

export function saveProfile(data: HealthProfile) {
  return http<HealthProfile>({ url: '/health/profile', method: 'put', data })
}

export function listRecords(params: { page?: number; pageSize?: number }) {
  return http<PageData<HealthRecord>>({ url: '/health/records', method: 'get', params })
}

export function createRecord(data: HealthRecord) {
  return http<{ id: string }>({ url: '/health/records', method: 'post', data })
}

export function deleteRecord(id: string) {
  return http<null>({ url: '/health/records/' + id, method: 'delete' })
}
