package cache

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// Store 对 go-redis 的薄封装：业务代码只依赖这里的方法，方便降级与打桩。
type Store struct {
	rdb *redis.Client
}

func New(rdb *redis.Client) *Store { return &Store{rdb: rdb} }

func (s *Store) Client() *redis.Client { return s.rdb }

// Get 返回值和是否存在；Redis 故障时返回 false，由调用方决定是否回源。
func (s *Store) Get(ctx context.Context, key string) (string, bool) {
	v, err := s.rdb.Get(ctx, key).Result()
	if err != nil {
		return "", false
	}
	return v, true
}

func (s *Store) Set(ctx context.Context, key, val string, ttl time.Duration) error {
	return s.rdb.Set(ctx, key, val, ttl).Err()
}

func (s *Store) Del(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	return s.rdb.Del(ctx, keys...).Err()
}

func (s *Store) Exists(ctx context.Context, key string) bool {
	n, err := s.rdb.Exists(ctx, key).Result()
	return err == nil && n > 0
}

// TryExists 区分“键不存在”与“Redis 故障”，让调用方在 Redis 降级时先放行核心业务。
func (s *Store) TryExists(ctx context.Context, key string) (bool, error) {
	if s.rdb == nil {
		return false, errors.New("redis client is nil")
	}
	n, err := s.rdb.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func (s *Store) SAdd(ctx context.Context, key string, members ...string) error {
	return s.rdb.SAdd(ctx, key, members).Err()
}

func (s *Store) SMembers(ctx context.Context, key string) []string {
	v, err := s.rdb.SMembers(ctx, key).Result()
	if err != nil {
		return nil
	}
	return v
}

// Incr 返回自增后的值；失败返回 -1。
func (s *Store) Incr(ctx context.Context, key string, ttl time.Duration) int64 {
	pipe := s.rdb.TxPipeline()
	inc := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		return -1
	}
	return inc.Val()
}

// GetJSON 反序列化缓存，miss 或脏数据返回 false。
func (s *Store) GetJSON(ctx context.Context, key string, dst any) bool {
	raw, ok := s.Get(ctx, key)
	if !ok {
		return false
	}
	return json.Unmarshal([]byte(raw), dst) == nil
}

func (s *Store) SetJSON(ctx context.Context, key string, val any, ttl time.Duration) error {
	b, err := json.Marshal(val)
	if err != nil {
		return err
	}
	return s.Set(ctx, key, string(b), ttl)
}

// Ping 启动自检。
func (s *Store) Ping(ctx context.Context) error {
	if s.rdb == nil {
		return errors.New("redis client is nil")
	}
	return s.rdb.Ping(ctx).Err()
}
