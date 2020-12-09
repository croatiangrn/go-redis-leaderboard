package go_redis_leaderboard

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

// RedisSettings stores Host and Password to connect to redis
type RedisSettings struct {
	Host     string
	Password string
}

type Leaderboard struct {
	RedisSettings   RedisSettings
	AppID           string
	EventType       string
	MetaData        string
}
