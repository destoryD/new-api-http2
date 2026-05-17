package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const (
	channelRPMLimitWindow = 60 * time.Second
	channelRPMLimitTTL    = 2 * time.Minute
)

var (
	channelRPMStore = struct {
		sync.Mutex
		items         map[string][]int64
		lastCleanupMs int64
	}{items: make(map[string][]int64)}
	channelRPMSeq uint64
)

type channelRPMRequest struct {
	key   string
	limit int
	scope string
}

func AllowChannelRPM(channelID int, modelName string, rpmLimit int, modelRPMLimit int) (bool, string, error) {
	if channelID <= 0 {
		return true, "", nil
	}

	requests := make([]channelRPMRequest, 0, 2)
	modelName = strings.TrimSpace(modelName)
	if modelRPMLimit > 0 && modelName != "" {
		requests = append(requests, channelRPMRequest{
			key:   fmt.Sprintf("channel_rpm:%d:model:%s", channelID, modelName),
			limit: modelRPMLimit,
			scope: "model",
		})
	}
	if rpmLimit > 0 {
		requests = append(requests, channelRPMRequest{
			key:   fmt.Sprintf("channel_rpm:%d", channelID),
			limit: rpmLimit,
			scope: "channel",
		})
	}
	if len(requests) == 0 {
		return true, "", nil
	}

	if common.RedisEnabled && common.RDB != nil {
		return allowChannelRPMRedis(context.Background(), requests)
	}

	return allowChannelRPMMemory(requests)
}

func allowChannelRPMMemory(requests []channelRPMRequest) (bool, string, error) {
	nowMs := time.Now().UnixMilli()
	cutoffMs := nowMs - channelRPMLimitWindow.Milliseconds()

	channelRPMStore.Lock()
	defer channelRPMStore.Unlock()

	if nowMs-channelRPMStore.lastCleanupMs >= channelRPMLimitTTL.Milliseconds() {
		cleanupChannelRPMStoreLocked(cutoffMs)
		channelRPMStore.lastCleanupMs = nowMs
	}

	for _, req := range requests {
		queue := channelRPMStore.items[req.key]
		queue = pruneChannelRPMQueue(queue, cutoffMs)
		channelRPMStore.items[req.key] = queue
		if len(queue) >= req.limit {
			return false, req.scope, nil
		}
	}

	for _, req := range requests {
		channelRPMStore.items[req.key] = append(channelRPMStore.items[req.key], nowMs)
	}
	return true, "", nil
}

func cleanupChannelRPMStoreLocked(cutoffMs int64) {
	for key, queue := range channelRPMStore.items {
		queue = pruneChannelRPMQueue(queue, cutoffMs)
		if len(queue) == 0 {
			delete(channelRPMStore.items, key)
		} else {
			channelRPMStore.items[key] = queue
		}
	}
}

func pruneChannelRPMQueue(queue []int64, cutoffMs int64) []int64 {
	keepFrom := 0
	for keepFrom < len(queue) && queue[keepFrom] <= cutoffMs {
		keepFrom++
	}
	if keepFrom == 0 {
		return queue
	}
	return queue[keepFrom:]
}

func allowChannelRPMRedis(ctx context.Context, requests []channelRPMRequest) (bool, string, error) {
	now := time.Now()
	nowMs := now.UnixMilli()
	cutoffMs := now.Add(-channelRPMLimitWindow).UnixMilli()
	member := fmt.Sprintf("%d-%d", nowMs, atomic.AddUint64(&channelRPMSeq, 1))
	keys := make([]string, 0, len(requests))
	args := make([]interface{}, 0, 4+len(requests))
	args = append(args, nowMs, cutoffMs, int64(channelRPMLimitTTL.Seconds()), member)
	for _, req := range requests {
		keys = append(keys, req.key)
		args = append(args, req.limit)
	}
	script := `
local n = #KEYS
for i = 1, n do
  redis.call("ZREMRANGEBYSCORE", KEYS[i], "-inf", ARGV[2])
  local count = redis.call("ZCARD", KEYS[i])
  if count >= tonumber(ARGV[4 + i]) then
    redis.call("EXPIRE", KEYS[i], ARGV[3])
    return i
  end
end
for i = 1, n do
  redis.call("ZADD", KEYS[i], ARGV[1], ARGV[4])
  redis.call("EXPIRE", KEYS[i], ARGV[3])
end
return 0
`
	result, err := common.RDB.Eval(
		ctx,
		script,
		keys,
		args...,
	).Int()
	if err != nil {
		return false, "", err
	}
	if result > 0 && result <= len(requests) {
		return false, requests[result-1].scope, nil
	}

	return true, "", nil
}
