package utils

import "os"

// GetEnvOrDefault gets environment variable with given key
// If it's not exists returns given default value
func GetEnvOrDefault(key, def string) string {
	value := os.Getenv(key)
	if value == "" {
		return def
	}
	return value
}
