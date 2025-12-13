package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	rdb *redis.Client
	ctx = context.Background()
)

// InitRedis 初始化Redis连接
func InitRedis() error {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}

	rdb = redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: "", // no password
		DB:       0,
	})

	// 测试连接
	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		return fmt.Errorf("Redis连接失败: %v", err)
	}

	fmt.Printf("Redis连接成功: %s\n", addr)
	return nil
}

// GetClient 获取Redis客户端
func GetClient() *redis.Client {
	return rdb
}

// Set 设置缓存
func Set(key string, value interface{}, expiration time.Duration) error {
	if rdb == nil {
		return fmt.Errorf("Redis未初始化")
	}

	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	return rdb.Set(ctx, key, data, expiration).Err()
}

// Get 获取缓存
func Get(key string, dest interface{}) error {
	if rdb == nil {
		return fmt.Errorf("Redis未初始化")
	}

	data, err := rdb.Get(ctx, key).Bytes()
	if err != nil {
		return err
	}

	return json.Unmarshal(data, dest)
}

// Delete 删除缓存
func Delete(key string) error {
	if rdb == nil {
		return fmt.Errorf("Redis未初始化")
	}

	return rdb.Del(ctx, key).Err()
}

// Exists 检查key是否存在
func Exists(key string) bool {
	if rdb == nil {
		return false
	}

	result, err := rdb.Exists(ctx, key).Result()
	if err != nil {
		return false
	}

	return result > 0
}

// Close 关闭Redis连接
func Close() error {
	if rdb != nil {
		return rdb.Close()
	}
	return nil
}
