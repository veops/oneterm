package db

import (
	"fmt"

	"github.com/veops/oneterm/internal/model"
)

// getMySQLConfig returns MySQL client configuration
func getMySQLConfig(ip string, port int, account *model.Account) DBClientConfig {
	args := []string{"-h", ip, "-P", fmt.Sprintf("%d", port), "-u", account.Account}
	if account.Password != "" {
		args = append(args, fmt.Sprintf("-p%s", account.Password))
	}

	return DBClientConfig{
		Command:     "mysql",
		Args:        args,
		ExitAliases: []string{"exit", "quit", "\\q"},
	}
}
