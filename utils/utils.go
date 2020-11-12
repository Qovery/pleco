package utils

import (
	"time"
)

func CheckIfExpired(creationTime time.Time, ttl int64) bool {
	expirationTime := creationTime.Add(time.Duration(ttl) * time.Second)
	return time.Now().After(expirationTime)
}