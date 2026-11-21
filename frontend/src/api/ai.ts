import { http } from '@/utils/request'
import type { AiMessage, PageData } from '@/types/api'

export interface Conversation {
  id: string
  title: string
  updatedAt: string
}

export function listConversations(params: { page?: number; pageSize?: number }) {
  return http<PageData<Conversation>>({ url: '/ai/conversations', method: 'get', params })
}

export function createConversation(title?: string) {
  return http<Conversation>({ url: '/ai/conversations', method: 'post', data: { title } })
}

export function listMessages(cid: string, params: { page?: number; pageSize?: number }) {
  return http<PageData<AiMessage>>({
    url: '/ai/conversations/' + cid + '/messages',
    method: 'get',
    params,
  })
}

export function chat(payload: { conversationId?: string; content: string }) {
  return http<{ conversationId: string; messageId: string; content: string }>({
    url: '/ai/chat',
    method: 'post',
    data: payload,
  })
}
