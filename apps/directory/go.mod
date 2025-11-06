module github.com/stonedem0/small-talk/apps/directory

go 1.23.0

require (
	github.com/golang-jwt/jwt/v5 v5.3.0
	github.com/joho/godotenv v1.5.1
	github.com/redis/go-redis/v9 v9.16.0
	github.com/stonedem0/small-talk/internal/shared v0.0.0
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
)

replace github.com/stonedem0/small-talk/internal/shared => ../../internal/shared
