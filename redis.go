package go_redis_leaderboard

import (
	"github.com/go-redis/redis/v8"
)

// RedisSettings stores Host, Password and DB to connect to redis
type RedisSettings struct {
	Host     string
	Password string
	DB       int
}

func connectToRedis(host string, password string, DB int) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     host,
		Password: password,
		DB:       DB,
	})
}
