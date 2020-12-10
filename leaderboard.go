package go_redis_leaderboard

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/go-redis/redis/v8"
)

const (
	DevMode        = "dev"
	StagingMode    = "staging"
	ProductionMode = "prod"

	PageSizeLimit = 100
)

var ctx = context.Background()

var (
	ErrAppIDEmpty                       = errors.New("leaderboard: empty app id")
	ErrEventTypeEmpty                   = errors.New("leaderboard: empty event type")
	ErrMetadataEmpty                    = errors.New("leaderboard: metadata empty")
	ErrIncrementByMustBePozitiveInteger = errors.New("leaderboard: incrementBy must be positive integer")
)

var allowedModes = map[string]bool{
	DevMode:        true,
	StagingMode:    true,
	ProductionMode: true,
}

// User will be used as a leaderboard item
type User struct {
	UserID string          `json:"user_id"`
	Score  int             `json:"score"`
	Rank   int             `json:"rank"`
	Info   json.RawMessage `json:"basic_info"`
}

// UserInfo consists of basic user info such as user id, username, avatar
type UserInfo struct {
	UserID string          `json:"user_id"`
	Data   json.RawMessage `json:"data"`
}

type Leaderboard struct {
	RedisSettings   RedisSettings
	AppID           string
	EventType       string
	MetaData        string
	mode            string
	redisCli        *redis.Client
	leaderboardName string
}

func NewLeaderboard(redisSettings RedisSettings, appID, eventType, metaData, mode, redisLeaderboardNameKey string) (*Leaderboard, error) {
	redisConn := connectToRedis(redisSettings.Host, redisSettings.Password, redisSettings.DB)
	if _, ok := allowedModes[mode]; !ok {
		mode = DevMode
	}

	if len(appID) == 0 {
		return nil, ErrAppIDEmpty
	}

	if len(eventType) == 0 {
		return nil, ErrEventTypeEmpty
	}

	if len(metaData) == 0 {
		return nil, ErrMetadataEmpty
	}

	// Leaderboard naming convention: "go_leaderboard-<mode>-<appID>-<eventType>-<metaData>"
	return &Leaderboard{RedisSettings: redisSettings, AppID: appID, EventType: eventType, MetaData: metaData, redisCli: redisConn, leaderboardName: redisLeaderboardNameKey}, nil
}

// UpsertMember inserts or updates member in leaderboard given
func (l *Leaderboard) UpsertMember(userID string, score int) (user User, err error) {
	member := &redis.Z{
		Score:  float64(score),
		Member: userID,
	}

	if _, err := l.redisCli.ZAdd(ctx, l.leaderboardName, member).Result(); err != nil {
		return User{}, err
	}

	// Returns the rank of member in the sorted set stored at key, with the scores ordered from high to low.
	// The rank (or index) is 0-based, which means that the member with the highest score has rank 0.
	rank, err := l.redisCli.ZRevRank(ctx, l.leaderboardName, userID).Result()
	if err != nil {
		return User{}, err
	}

	u := User{
		UserID: userID,
		Score:  score,
		Rank:   int(rank) + 1,
		Info:   nil,
	}

	return u, nil
}

func (l *Leaderboard) GetMember(userID string) (user User, err error) {
	rank, err := getMemberRank(l.redisCli, l.leaderboardName, userID)
	if err != nil {
		return User{}, err
	}

	score, err := getMemberScore(l.redisCli, l.leaderboardName, userID)
	if err != nil {
		return User{}, err
	}

	user = User{
		UserID: userID,
		Score:  score,
		Rank:   rank,
		Info:   nil,
	}

	return
}

func (l *Leaderboard) IncrementMemberScore(userID string, incrementBy int) (user User, err error) {
	rank, err := getMemberRank(l.redisCli, l.leaderboardName, userID)
	if err != nil {
		return User{}, err
	}

	newScore, err := incrementMemberScore(l.redisCli, l.leaderboardName, userID, incrementBy)
	if err != nil {
		return User{}, err
	}

	user = User{
		UserID: userID,
		Score:  newScore,
		Rank:   rank,
		Info:   nil,
	}

	return user, nil
}

func (l *Leaderboard) UpsertMemberInfo(info UserInfo) (updatedInfo UserInfo, err error) {
	return
}

// Returns the rank of member in the sorted set stored at key, with the scores ordered from high to low.
func getMemberRank(redisCli *redis.Client, leaderboardName, userID string) (rank int, err error) {
	rankInt64, err := redisCli.ZRevRank(ctx, leaderboardName, userID).Result()
	if err != nil {
		return 0, err
	}

	return int(rankInt64) + 1, nil
}

func getMemberScore(redisCli *redis.Client, leaderboardName, userID string) (score int, err error) {
	floatScore, err := redisCli.ZScore(ctx, leaderboardName, userID).Result()
	if err != nil {
		return 0, err
	}

	return int(floatScore), nil
}

func incrementMemberScore(redisCli *redis.Client, leaderboardName, userID string, incrementBy int) (newScore int, err error) {
	if incrementBy < 0 {
		return 0, ErrIncrementByMustBePozitiveInteger
	}

	res, err := redisCli.ZIncrBy(ctx, leaderboardName, float64(incrementBy), userID).Result()
	if err != nil {
		return 0, err
	}

	return int(res), nil
}
