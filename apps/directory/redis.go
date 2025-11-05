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

func Init() {
	RDB = redis.NewClient(&redis.Options{
		Addr:     os.Getenv("REDIS_ADDR"),
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       0,
	})
	if err := RDB.Ping(ctx).Err(); err != nil {
		log.Fatalf("Could not connect to Redis: %v", err)
	}
	log.Println("Connected to Redis!")
}

func Owner(room string) (string, error) {
	val, err := RDB.Get(ctx, key(room)).Result()
	if err == redis.Nil {
		return "", nil
	}
	return val, err
}

func TryClaim(room, appID string) (bool, error) {
	return RDB.SetNX(ctx, key(room), appID, leaseTTL).Result()
}

func RefreshLease(room, appID string) error {
	cur, err := Owner(room)
	if err != nil {
		return err
	}
	if cur != appID {
		return nil
	}
	_, err = RDB.Expire(ctx, key(room), leaseTTL).Result()
	return err
}

func Release(room, appID string) error {
	cur, err := Owner(room)
	if err != nil {
		return err
	}
	if cur != appID {
		return nil
	}
	_, err = RDB.Del(ctx, key(room)).Result()
	return err
}
