package redis

import (
	"context"

	"github.com/redis/go-redis/v9"
)

func NewRedisClient(restURL, token string) *redis.Client {
	// Upstash Redis REST yerine native Redis protokolü de kullanılabilir.
	// Şimdilik basit bir client iskeleti bırakıyorum.
	opt, err := redis.ParseURL("rediss://default:" + token + "@" + extractHost(restURL) + ":6379")
	if err != nil {
		// fallback
		return redis.NewClient(&redis.Options{
			Addr: "localhost:6379",
		})
	}
	return redis.NewClient(opt)
}

func extractHost(restURL string) string {
	// https://xxxx.upstash.io → xxxx.upstash.io
	if len(restURL) > 8 {
		return restURL[8:]
	}
	return restURL
}

func Ping(ctx context.Context, rdb *redis.Client) error {
	return rdb.Ping(ctx).Err()
}
