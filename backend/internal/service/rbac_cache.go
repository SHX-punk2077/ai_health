package service

import (
	"context"
	"fmt"

	"ai_health/internal/consts"
	"ai_health/internal/pkg/cache"
)

// invalidateRBAC 删除一批用户的角色 / 权限缓存。
// 角色、菜单、用户角色关系变更后必须调用，否则最长 10 分钟内仍是旧权限。
func invalidateRBAC(ctx context.Context, store *cache.Store, uids []uint64) {
	if store == nil || len(uids) == 0 {
		return
	}
	seen := make(map[uint64]struct{}, len(uids))
	keys := make([]string, 0, len(uids)*2)
	for _, uid := range uids {
		if _, ok := seen[uid]; ok {
			continue
		}
		seen[uid] = struct{}{}
		keys = append(keys, fmt.Sprintf(consts.KeyPermissions, uid), fmt.Sprintf(consts.KeyRoles, uid))
	}
	_ = store.Del(ctx, keys...)
}

// uniqueIDs 去重并保持原顺序；空输入返回非 nil 空切片，便于 JSON 输出 []。
func uniqueIDs(ids []uint64) []uint64 {
	out := make([]uint64, 0, len(ids))
	seen := make(map[uint64]struct{}, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}
