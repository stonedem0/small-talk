package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	leaseTTL = 60 * time.Second
)

var RDB *redis.Client
var ctx = context.Background()

func key(room string) string { return "directory:room:" + room }

func RedisInit() {
	RDB = redis.NewClient(&redis.Options{
		Addr:     os.Getenv("REDIS_ADDR"),
		Password: os.Getenv("REDIS_PASSWORD"),
		Username: os.Getenv("REDIS_USERNAME"),
		DB:       0,
	})
	if err := RDB.Ping(ctx).Err(); err != nil {
		log.Fatalf("Could not connect to Redis: %v", err)
	}
	log.Println("Connected to Redis!")
}

// refreshLeaseScript atomically refreshes the TTL only if the caller owns the key.
var refreshLeaseScript = redis.NewScript(`
if redis.call('GET', KEYS[1]) == ARGV[1] then
    return redis.call('EXPIRE', KEYS[1], ARGV[2])
end
return 0
`)

// releaseScript atomically deletes the key only if the caller owns it.
var releaseScript = redis.NewScript(`
if redis.call('GET', KEYS[1]) == ARGV[1] then
    return redis.call('DEL', KEYS[1])
end
return 0
`)

func Owner(room string) (string, error) {
	val, err := RDB.Get(ctx, key(room)).Result()
	if err == redis.Nil {
		return "", nil
	}
	return val, err
}

func TryClaim(room, appID string) (bool, error) {
	ok, err := RDB.SetNX(ctx, key(room), appID, leaseTTL).Result()
	switch {
	case err != nil:
		leaseClaimsTotal.WithLabelValues("error").Inc()
	case ok:
		leaseClaimsTotal.WithLabelValues("success").Inc()
	default:
		leaseClaimsTotal.WithLabelValues("conflict").Inc()
	}
	return ok, err
}

func RefreshLease(room, appID string) error {
	ttlSecs := int(leaseTTL.Seconds())
	err := refreshLeaseScript.Run(ctx, RDB, []string{key(room)}, appID, ttlSecs).Err()
	if err != nil {
		leaseRefreshesTotal.WithLabelValues("error").Inc()
	} else {
		leaseRefreshesTotal.WithLabelValues("ok").Inc()
	}
	return err
}

func Release(room, appID string) error {
	err := releaseScript.Run(ctx, RDB, []string{key(room)}, appID).Err()
	if err == redis.Nil {
		leaseReleasesTotal.WithLabelValues("ok").Inc()
		return nil
	}
	if err != nil {
		leaseReleasesTotal.WithLabelValues("error").Inc()
		return err
	}
	leaseReleasesTotal.WithLabelValues("ok").Inc()
	return nil
}
