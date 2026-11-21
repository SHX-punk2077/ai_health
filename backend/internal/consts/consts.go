package consts

// 角色编码，与 sys_role.role_code 种子数据一致。
const (
	RoleSuperAdmin = "super_admin"
	RoleAdmin      = "admin"
	RoleUser       = "user"
)

// 账号 / 通用状态。
const (
	StatusDisabled = 0
	StatusEnabled  = 1
)

// Redis key 前缀与模板，禁止在业务代码里硬编码字符串。
const (
	KeyAccessToken  = "aihealth:auth:access:%d:%s"
	KeyRefreshToken = "aihealth:auth:refresh:%d:%s"
	KeyUserJTIs     = "aihealth:auth:uid_jti:%d"
	KeyPermissions  = "aihealth:perm:%d"
	KeyRoles        = "aihealth:role:%d"
	KeyLoginFail    = "aihealth:login:fail:%s"
	KeyProfile      = "aihealth:cache:profile:%d"
	KeySportTypes   = "aihealth:cache:sport:types"
	KeyMenuTree     = "aihealth:cache:menu:tree"
	KeyAIContext    = "aihealth:ai:ctx:%d"
	KeyAILock       = "aihealth:ai:lock:%d"
	KeyRateLimit    = "aihealth:rl:%s:%s"
)

// Gin context key。
const (
	CtxRequestID = "requestID"
	CtxUID       = "uid"
	CtxRoles     = "roles"
	CtxPerms     = "perms"
)
