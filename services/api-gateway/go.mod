module github.com/shivamrajput177/ai-meeting-intelligence/services/api-gateway

go 1.25.0

require (
	github.com/redis/go-redis/v9 v9.22.0
	github.com/shivamrajput177/ai-meeting-intelligence/shared v0.0.0-00010101000000-000000000000
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/golang-jwt/jwt/v5 v5.3.1 // indirect
	github.com/google/uuid v1.6.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
)

replace github.com/shivamrajput177/ai-meeting-intelligence/shared => ../../shared
