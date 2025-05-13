package db

import (
	"fmt"

	"github.com/veops/oneterm/internal/model"
)

// getRedisConfig returns Redis client configuration
func getRedisConfig(ip string, port int, account *model.Account) DBClientConfig {
	args := []string{"-h", ip, "-p", fmt.Sprintf("%d", port)}
	if account.Password != "" {
		args = append(args, "-a", account.Password)
	}

	return DBClientConfig{
		Command:     "redis-cli",
		Args:        args,
		ExitAliases: []string{"exit", "quit"},
	}
}
