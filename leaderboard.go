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
	ErrIncrementByMustBePositiveInteger = errors.New("leaderboard: incrementBy must be positive integer")
)

var allowedModes = map[string]bool{
	DevMode:        true,
	StagingMode:    true,
	ProductionMode: true,
}

// User will be used as a leaderboard item
type User struct {
	UserID string   `json:"user_id"`
	Score  int      `json:"score"`
	Rank   int      `json:"rank"`
	Info   UserInfo `json:"basic_info"`
}

// UserInfo consists of basic user info such as user id, username, avatar
type UserInfo struct {
	UserID string          `json:"user_id"`
	Data   json.RawMessage `json:"data"`
}

type Leaderboard struct {
	RedisSettings    RedisSettings
	mode             string
	redisCli         *redis.Client
	leaderboardName  string
	userInfoHashName string
}

// NewLeaderboard is constructor for Leaderboard.
//
// IMPORTANT: ``leaderboardName`` and ``uniqueIdentifier`` must be unique project/app wide!
//
// uniqueIdentifier is something like table name that will be used to store user info.
func NewLeaderboard(redisSettings RedisSettings, mode, leaderboardName, userInfoStorageHash string) (*Leaderboard, error) {
	redisConn := connectToRedis(redisSettings.Host, redisSettings.Password, redisSettings.DB)
	if _, ok := allowedModes[mode]; !ok {
		mode = DevMode
	}

	// Leaderboard naming convention: "go_leaderboard-<mode>-<appID>-<eventType>-<metaData>"
	return &Leaderboard{RedisSettings: redisSettings, redisCli: redisConn, leaderboardName: leaderboardName, userInfoHashName: userInfoStorageHash}, nil
}

// InsertMember inserts member to leaderboard if the member doesn't exist
func (l *Leaderboard) FirstOrInsertMember(userID string, score int) (user User, err error) {
	currentRank, err := getMemberRank(l.redisCli, l.leaderboardName, userID)
	if err != nil && !errors.Is(err, redis.Nil) {
		return User{}, err
	}

	// Member already exists in our leaderboard, fetch score and info, too and return the data
	if currentRank > 0 {
		currentScore, err := getMemberScore(l.redisCli, l.leaderboardName, userID)
		if err != nil {
			return User{}, err
		}

		user = User{
			UserID: userID,
			Score:  currentScore,
			Rank:   currentRank,
		}

		return user, nil
	}

	// Member doesn't exist. Insert rank, score and info and return the data
	if err := insertMemberScore(l.redisCli, l.leaderboardName, userID, score); err != nil {
		return User{}, err
	}

	rank, err := updateMemberRank(l.redisCli, l.leaderboardName, userID)
	if err != nil {
		return User{}, err
	}

	u := User{
		UserID: userID,
		Score:  score,
		Rank:   rank,
	}

	return u, nil
}

func (l *Leaderboard) GetMember(userID string, withInfo bool) (user User, err error) {
	rank, err := getMemberRank(l.redisCli, l.leaderboardName, userID)
	if err != nil {
		return User{}, err
	}

	score, err := getMemberScore(l.redisCli, l.leaderboardName, userID)
	if err != nil {
		return User{}, err
	}

	var info UserInfo
	if withInfo {
		userInfo, err := l.GetMemberInfo(userID)
		if err != nil {
			if !errors.Is(err, redis.Nil) {
				return User{}, err
			}
		
			userInfo.UserID = userID
		}

		info = userInfo
	}

	user = User{
		UserID: userID,
		Score:  score,
		Rank:   rank,
		Info:   info,
	}

	return
}

func (l *Leaderboard) IncrementMemberScore(userID string, incrementBy int) (user User, err error) {
	newScore, err := incrementMemberScore(l.redisCli, l.leaderboardName, userID, incrementBy)
	if err != nil {
		return User{}, err
	}

	rank, err := updateMemberRank(l.redisCli, l.leaderboardName, userID)
	if err != nil {
		return User{}, err
	}

	user = User{
		UserID: userID,
		Score:  newScore,
		Rank:   rank,
	}

	return user, nil
}

func (l *Leaderboard) GetMemberInfo(userID string) (info UserInfo, err error) {
	info.UserID = userID
	stringifiedData, err := l.redisCli.HGet(ctx, l.userInfoHashName, userID).Result()
	if err != nil {
		return UserInfo{}, err
	}

	if err := json.Unmarshal([]byte(stringifiedData), &info.Data); err != nil {
		return UserInfo{}, err
	}

	return info, nil
}

func (l *Leaderboard) UpsertMemberInfo(info UserInfo) error {
	if _, err := l.redisCli.HSet(ctx, info.UserID, info.Data).Result(); err != nil {
		return err
	}

	return nil
}

// Returns the rank of member in the sorted set stored at key,
// with the scores ordered from high to low starting from one.
func getMemberRank(redisCli *redis.Client, leaderboardName, userID string) (rank int, err error) {
	rankInt64, err := redisCli.ZRevRank(ctx, leaderboardName, userID).Result()
	if err != nil {
		return 0, err
	}

	return int(rankInt64) + 1, nil
}

func updateMemberRank(redisCli *redis.Client, leaderboardName, userID string) (rank int, err error) {
	// Returns the rank of member in the sorted set stored at key, with the scores ordered from high to low.
	// The rank (or index) is 0-based, which means that the member with the highest score has rank 0.
	res, err := redisCli.ZRevRank(ctx, leaderboardName, userID).Result()
	if err != nil {
		return 0, err
	}

	return int(res) + 1, nil
}

func getMemberScore(redisCli *redis.Client, leaderboardName, userID string) (score int, err error) {
	floatScore, err := redisCli.ZScore(ctx, leaderboardName, userID).Result()
	if err != nil {
		return 0, err
	}

	return int(floatScore), nil
}

func insertMemberScore(redisCli *redis.Client, leaderboardName, userID string, score int) error {
	member := &redis.Z{
		Score:  float64(score),
		Member: userID,
	}

	_, err := redisCli.ZAdd(ctx, leaderboardName, member).Result()
	if err != nil {
		return err
	}

	return nil
}

func incrementMemberScore(redisCli *redis.Client, leaderboardName, userID string, incrementBy int) (newScore int, err error) {
	if incrementBy < 0 {
		return 0, ErrIncrementByMustBePositiveInteger
	}

	res, err := redisCli.ZIncrBy(ctx, leaderboardName, float64(incrementBy), userID).Result()
	if err != nil {
		return 0, err
	}

	return int(res), nil
}
