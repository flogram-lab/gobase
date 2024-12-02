package gobase

import (
	"log"
	"os"
	"strconv"

	"github.com/go-faster/errors"
)

// Wrapper for os.Getenv (may trigger log.Fatal if value is empty)
func GetenvStr(key string, defaultValue string, allowNull bool) string {
	val := os.Getenv(key)

	if val == "" {
		if !allowNull {
			log.Fatalf("Required environment variable %s is not set or empty!", key)
		}
		return defaultValue
	}

	return val
}

// Wrapper for os.Getenv (may trigger log.Fatal if value is empty or is not valid integer)
func GetenvInt(key string, defaultValue int, allowNull bool) int {
	val := os.Getenv(key)

	if val == "" {
		if !allowNull {
			log.Fatalf("Required environment variable %s is not set or empty!", key)
		}
		return defaultValue
	}

	i, err := strconv.Atoi(val)
	if err != nil {
		log.Fatal(errors.Wrap(err, " parse app id"))
	}

	return i
}

// Using os.Getenv, get map of key=value strings from environment
func GetenvMap(keys ...string) map[string]any {
	v := make(map[string]any, len(keys))
	for i := range keys {
		key := keys[i]
		v[key] = os.Getenv(key)
	}
	return v
}
