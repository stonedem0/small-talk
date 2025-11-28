package main

import (
	"context"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
)

var RDB *redis.Client

func init() {
	// Try to load env from app.env (prod) then .env (local); ok if missing when env vars are injected.
	if err := godotenv.Load("app.env"); err != nil {
		_ = godotenv.Load()
	}
}

func InitRedis() {
	RDB = redis.NewClient(&redis.Options{
		Addr:     os.Getenv("REDIS_ADDR"),
		Username: os.Getenv("REDIS_USERNAME"),
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       0,
	})
	if err := RDB.Ping(ctx).Err(); err != nil {
		log.Fatalf("Could not connect to Redis: %v", err)
	}
	log.Println("Connected to Redis!")
}

func PublishMessage(room, message string) error {
	ctx := context.Background()
	return RDB.Publish(ctx, "room:"+room, message).Err()
}
