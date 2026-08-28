package coordination

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type TokenState struct {
	ID       string `json:"id"`
	InFlight int64  `json:"in_flight"`
	Healthy  bool   `json:"healthy"`
}

type localWindow struct {
	started time.Time
	count   int
}

type stickyToken struct {
	id        string
	expiresAt time.Time
}

type Coordinator struct {
	client    *redis.Client
	prefix    string
	mu        sync.Mutex
	windows   map[string]localWindow
	sticky    map[string]stickyToken
	inflight  map[string]int64
	unhealthy map[string]time.Time
	warned    bool
}

func New(redisURL, prefix string) (*Coordinator, error) {
	c := &Coordinator{prefix: prefix, windows: map[string]localWindow{}, sticky: map[string]stickyToken{}, inflight: map[string]int64{}, unhealthy: map[string]time.Time{}}
	if redisURL == "" {
		return c, nil
	}
	options, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse REDIS_URL: %w", err)
	}
	c.client = redis.NewClient(options)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := c.client.Ping(ctx).Err(); err != nil {
		log.Printf("redis unavailable at startup; using local fallback: %v", err)
	}
	return c, nil
}

func (c *Coordinator) Close() error {
	if c.client != nil {
		return c.client.Close()
	}
	return nil
}

func (c *Coordinator) RedisAvailable(ctx context.Context) bool {
	if c.client == nil {
		return false
	}
	check, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	return c.client.Ping(check).Err() == nil
}

func (c *Coordinator) Allow(ctx context.Context, scope, subject string, limit int, window time.Duration, now time.Time) bool {
	if c.client != nil {
		bucket := now.UTC().Truncate(window)
		key := c.key("limit", scope, subject, strconv.FormatInt(bucket.Unix(), 10))
		count, err := c.client.Incr(ctx, key).Result()
		if err == nil {
			if count == 1 {
				_ = c.client.Expire(ctx, key, window+time.Minute).Err()
			}
			return count <= int64(limit)
		}
		c.warnFallback(err)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	key := scope + "|" + subject
	started := now.UTC().Truncate(window)
	current := c.windows[key]
	if !current.started.Equal(started) {
		current = localWindow{started: started}
	}
	if current.count >= limit {
		return false
	}
	current.count++
	c.windows[key] = current
	for name, value := range c.windows {
		if value.started.Before(started) {
			delete(c.windows, name)
		}
	}
	return true
}

func (c *Coordinator) AcquireToken(ctx context.Context, subject string, candidates []string, exclude string, stickyTTL time.Duration, now time.Time) (string, bool) {
	if len(candidates) == 0 {
		return "", false
	}
	if c.client != nil {
		if token, ok, err := c.acquireRedis(ctx, subject, candidates, exclude, stickyTTL); err == nil {
			return token, ok
		} else {
			c.warnFallback(err)
		}
	}
	return c.acquireLocal(subject, candidates, exclude, stickyTTL, now)
}

func (c *Coordinator) ReleaseToken(ctx context.Context, id string) {
	if id == "" {
		return
	}
	if c.client != nil {
		key := c.key("inflight", id)
		value, err := c.client.Decr(ctx, key).Result()
		if err == nil {
			if value <= 0 {
				_ = c.client.Del(ctx, key).Err()
			}
			return
		}
		c.warnFallback(err)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.inflight[id] > 1 {
		c.inflight[id]--
	} else {
		delete(c.inflight, id)
	}
}

func (c *Coordinator) MarkUnhealthy(ctx context.Context, id string, cooldown time.Duration, now time.Time) {
	if c.client != nil {
		if err := c.client.Set(ctx, c.key("unhealthy", id), "1", cooldown).Err(); err == nil {
			return
		} else {
			c.warnFallback(err)
		}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.unhealthy[id] = now.Add(cooldown)
}

func (c *Coordinator) States(ctx context.Context, ids []string, now time.Time) []TokenState {
	states := make([]TokenState, 0, len(ids))
	for _, id := range ids {
		state := TokenState{ID: id, Healthy: true}
		if c.client != nil {
			pipe := c.client.Pipeline()
			inflight := pipe.Get(ctx, c.key("inflight", id))
			unhealthy := pipe.Exists(ctx, c.key("unhealthy", id))
			if _, err := pipe.Exec(ctx); err == nil || err == redis.Nil {
				state.InFlight, _ = inflight.Int64()
				state.Healthy = unhealthy.Val() == 0
				states = append(states, state)
				continue
			}
		}
		c.mu.Lock()
		state.InFlight = c.inflight[id]
		state.Healthy = !c.unhealthy[id].After(now)
		c.mu.Unlock()
		states = append(states, state)
	}
	return states
}

func (c *Coordinator) acquireRedis(ctx context.Context, subject string, candidates []string, exclude string, stickyTTL time.Duration) (string, bool, error) {
	stickyKey := c.key("sticky", subject)
	sticky, err := c.client.Get(ctx, stickyKey).Result()
	if err != nil && err != redis.Nil {
		return "", false, err
	}
	contains := func(id string) bool {
		if id == "" || id == exclude {
			return false
		}
		for _, candidate := range candidates {
			if candidate == id {
				return true
			}
		}
		return false
	}
	if contains(sticky) {
		healthy, err := c.client.Exists(ctx, c.key("unhealthy", sticky)).Result()
		if err != nil {
			return "", false, err
		}
		if healthy == 0 {
			if err := c.client.Incr(ctx, c.key("inflight", sticky)).Err(); err != nil {
				return "", false, err
			}
			return sticky, true, nil
		}
	}
	type load struct {
		id    string
		count int64
	}
	loads := make([]load, 0, len(candidates))
	for _, id := range candidates {
		if id == exclude {
			continue
		}
		if unhealthy, err := c.client.Exists(ctx, c.key("unhealthy", id)).Result(); err != nil {
			return "", false, err
		} else if unhealthy > 0 {
			continue
		}
		count, err := c.client.Get(ctx, c.key("inflight", id)).Int64()
		if err != nil && err != redis.Nil {
			return "", false, err
		}
		loads = append(loads, load{id, count})
	}
	if len(loads) == 0 {
		return "", false, nil
	}
	sort.SliceStable(loads, func(i, j int) bool { return loads[i].count < loads[j].count })
	selected := loads[0].id
	pipe := c.client.TxPipeline()
	pipe.Incr(ctx, c.key("inflight", selected))
	pipe.Set(ctx, stickyKey, selected, stickyTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		return "", false, err
	}
	return selected, true, nil
}

func (c *Coordinator) acquireLocal(subject string, candidates []string, exclude string, stickyTTL time.Duration, now time.Time) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	sticky := c.sticky[subject]
	if sticky.id != "" && sticky.expiresAt.After(now) && sticky.id != exclude && contains(candidates, sticky.id) && !c.unhealthy[sticky.id].After(now) {
		c.inflight[sticky.id]++
		return sticky.id, true
	}
	delete(c.sticky, subject)
	selected := ""
	var lowest int64
	for _, id := range candidates {
		if id == exclude || c.unhealthy[id].After(now) {
			continue
		}
		if selected == "" || c.inflight[id] < lowest {
			selected, lowest = id, c.inflight[id]
		}
	}
	if selected == "" {
		return "", false
	}
	c.sticky[subject] = stickyToken{id: selected, expiresAt: now.Add(stickyTTL)}
	c.inflight[selected]++
	return selected, true
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func (c *Coordinator) key(parts ...string) string {
	value := c.prefix
	for _, part := range parts {
		value += ":" + part
	}
	return value
}

func (c *Coordinator) warnFallback(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.warned {
		log.Printf("redis operation failed; using local fallback: %v", err)
		c.warned = true
	}
}
