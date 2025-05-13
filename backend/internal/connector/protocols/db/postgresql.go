package db

import (
	"fmt"
	"os"
	"strings"

	"github.com/veops/oneterm/internal/model"
)

// getPostgreSQLConfig returns PostgreSQL client configuration
func getPostgreSQLConfig(ip string, port int, account *model.Account) DBClientConfig {
	// Set PGPASSWORD environment variable instead of using -W
	os.Setenv("PGPASSWORD", account.Password)

	args := []string{
		"-h", ip,
		"-p", fmt.Sprintf("%d", port),
		"-U", account.Account,
		"postgres", // Default database name
	}

	// Add database name if specified in the account.Account field (username/database format)
	parts := strings.Split(account.Account, "/")
	if len(parts) > 1 {
		args = append(args, "-d", parts[1])
	}

	return DBClientConfig{
		Command:     "psql",
		Args:        args,
		ExitAliases: []string{"\\q", "exit", "quit"},
	}
}
