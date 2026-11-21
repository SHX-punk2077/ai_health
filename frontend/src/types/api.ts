// 与后端 internal/pkg/response 保持一致
export interface ApiBody<T = unknown> {
  code: number
  message: string
  data: T
  requestId?: string
}

export interface PageData<T> {
  list: T[]
  total: number
  page: number
  pageSize: number
}

export interface Token {
  accessToken: string
  refreshToken: string
  tokenType: string
  expiresIn: number
  refreshExpiresIn: number
}

export interface UserInfo {
  /** 后端把 BIGINT 序列化成字符串，避免 JS 精度丢失 */
  id: string
  username: string
  nickname: string
  avatar: string
  email: string
  phone: string
  status: number
  roles: string[]
  permissions: string[]
}

export interface LoginResult extends Token {
  user: UserInfo
}

export interface HealthProfile {
  id?: string
  userId?: string
  realName?: string
  age?: number
  gender?: number
  heightCm?: number
  weightKg?: number
  bloodSugarMmol?: number
  systolicPressure?: number
  diastolicPressure?: number
  totalCholesterolMmol?: number
  heartRate?: number
  sleepDurationHours?: number
  bmi?: number
}

export interface AiMessage {
  id: string
  conversationId: string
  messageRole: 'system' | 'user' | 'assistant'
  content: string
  sequenceNo: number
  createdAt: string
}

export interface HealthRecord extends HealthProfile {
  id: string
  measuredAt: string
  /** 记录类型：1=日常 2=体检 3=运动后 …（后端常量待定） */
  recordType?: number
}
