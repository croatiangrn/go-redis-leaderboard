package go_redis_leaderboard

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/go-redis/redis/v8"
	"math"
	"strconv"
)

const (
	DevMode        = "dev"
	StagingMode    = "staging"
	ProductionMode = "prod"

	UnrankedMember  = -1
	DefaultPageSize = 25
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

var allowedPageSizes = map[int]bool{
	10:  true,
	25:  true,
	50:  true,
	100: true,
}

// User will be used as a leaderboard item
type User struct {
	UserID         string          `json:"user_id"`
	Score          int             `json:"score"`
	Rank           int             `json:"rank"`
	AdditionalInfo json.RawMessage `json:"additional_info"`
}

type Leaderboard struct {
	RedisSettings    RedisSettings
	PageSize         int
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
//goland:noinspection GoUnusedExportedFunction
func NewLeaderboard(redisSettings RedisSettings, mode, leaderboardName, userInfoStorageHash string, pageSize int) (*Leaderboard, error) {
	redisConn := connectToRedis(redisSettings.Host, redisSettings.Password, redisSettings.DB)
	if _, ok := allowedModes[mode]; !ok {
		mode = DevMode
	}

	if _, ok := allowedPageSizes[pageSize]; !ok {
		pageSize = DefaultPageSize
	}

	// Leaderboard naming convention: "go_leaderboard-<mode>-<appID>-<eventType>-<metaData>"
	return &Leaderboard{RedisSettings: redisSettings, redisCli: redisConn, leaderboardName: leaderboardName, userInfoHashName: userInfoStorageHash, PageSize: pageSize}, nil
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
		if !errors.Is(err, redis.Nil) {
			return User{}, err
		}

		rank = UnrankedMember
	}

	var score int
	var additionalInfo json.RawMessage

	if rank != UnrankedMember {
		memberScore, scoreErr := getMemberScore(l.redisCli, l.leaderboardName, userID)
		if scoreErr != nil {
			if !errors.Is(err, redis.Nil) {
				return User{}, err
			}
		}

		score = memberScore
		if withInfo {
			message, err := l.GetMemberInfo(userID)
			if err != nil {
				if !errors.Is(err, redis.Nil) {
					return User{}, err
				}
			}

			additionalInfo = message
		}
	}

	user = User{
		UserID:         userID,
		Score:          score,
		Rank:           rank,
		AdditionalInfo: additionalInfo,
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

func (l *Leaderboard) GetMemberInfo(userID string) (bytes []byte, err error) {
	stringifiedData, err := l.redisCli.HGet(ctx, l.userInfoHashName, userID).Result()
	if err != nil {
		return nil, err
	}

	unquotedText, _ := strconv.Unquote(stringifiedData)
	raw, err := base64.StdEncoding.DecodeString(unquotedText)
	if err != nil {
		return nil, err
	}

	return raw, nil
}

type AdditionalUserInfo json.RawMessage

func (a *AdditionalUserInfo) MarshalBinary() ([]byte, error) {
	return json.Marshal(a)
}

func (a *AdditionalUserInfo) UnmarshalBinary(data []byte) error {
	return json.Unmarshal(data, a)
}

func (l *Leaderboard) UpsertMemberInfo(userID string, additionalData AdditionalUserInfo) error {
	data, err := json.Marshal(&additionalData)
	if err != nil {
		return err
	}

	if _, err := l.redisCli.HSet(ctx, l.userInfoHashName, userID, string(data)).Result(); err != nil {
		return err
	}

	return nil
}

func (l *Leaderboard) TotalMembers() (int, error) {
	members, err := l.redisCli.ZCard(ctx, l.leaderboardName).Result()
	if err != nil {
		return 0, err
	}

	return int(members), nil
}

func (l *Leaderboard) TotalPages() int {
	pages := 0

	total, err := l.redisCli.ZCount(ctx, l.leaderboardName, "-inf", "+inf").Result()
	if err == nil {
		pages = int(math.Ceil(float64(total) / float64(l.PageSize)))
	}

	return pages
}

func (l *Leaderboard) GetLeaders(page int) ([]User, error) {
	if page < 1 {
		page = 1
	}

	if page > l.TotalPages() {
		page = l.TotalPages()
	}

	redisIndex := page - 1
	startOffset := redisIndex * l.PageSize
	if startOffset < 0 {
		startOffset = 0
	}
	endOffset := (startOffset + l.PageSize) - 1

	return getMembersByRange(l.redisCli, l.leaderboardName, l.PageSize, startOffset, endOffset)
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

func getMembersByRange(redisCli *redis.Client, leaderboard string, pageSize int, startOffset int, endOffset int) ([]User, error) {
	users := make([]User, pageSize)

	values, err := redisCli.ZRevRangeWithScores(ctx, leaderboard, int64(startOffset), int64(endOffset)).Result()
	if err != nil {
		return nil, err
	}

	for i := range values {
		userID := values[i].Member.(string)

		rank, err := getMemberRank(redisCli, leaderboard, userID)
		if err != nil {
			return nil, err
		}

		score, err := getMemberScore(redisCli, leaderboard, userID)
		if err != nil {
			return nil, err
		}

		user := User{
			UserID:         userID,
			Score:          score,
			Rank:           rank,
			AdditionalInfo: nil,
		}

		users = append(users, user)
	}

	return users, nil
}
