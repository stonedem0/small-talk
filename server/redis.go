package main

import (
	"context"
	"log"

	"github.com/redis/go-redis/v9"
)

// var ctx = context.Background()

var RDB *redis.Client

func InitRedis() {
	RDB = redis.NewClient(&redis.Options{
		Addr:     "127.0.0.1:6379",
		Password: "cCq!qRG7iMdZKdUg_r*kY",
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

func SubscribeToRoom(room string) {
	ctx := context.Background()
	pubsub := RDB.Subscribe(ctx, "room:"+room)
	defer pubsub.Close()

	ch := pubsub.Channel()
	for msg := range ch {
		log.Printf("[Room %s] %s\n", room, msg.Payload)
	}
}
