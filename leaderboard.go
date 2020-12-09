package go_redis_leaderboard

import "github.com/go-redis/redis/v8"

// User will be used as a leaderboard item
type User struct {
	UserID string `json:"user_id"`
	Score  int    `json:"score"`
	Rank   int    `json:"rank"`
}

// UserInfo consists of basic user info such as user id, username, avatar
type UserInfo struct {
	UserID     string `json:"user_id"`
	UserName   string `json:"user_name"`
	UserAvatar string `json:"user_avatar"`
}

type Leaderboard struct {
	RedisSettings RedisSettings
	AppID         string
	EventType     string
	MetaData      string
	redisConn     *redis.Client
}

func NewLeaderboard(redisSettings RedisSettings, appID, eventType, metaData string) *Leaderboard {
	redisConn := connectToRedis(redisSettings.Host, redisSettings.Password, redisSettings.DB)
	return &Leaderboard{RedisSettings: redisSettings, AppID: appID, EventType: eventType, MetaData: metaData, redisConn: redisConn}
}
