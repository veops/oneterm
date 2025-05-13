package db

import (
	"fmt"

	"github.com/veops/oneterm/internal/model"
)

// getMongoDBConfig returns MongoDB client configuration
func getMongoDBConfig(ip string, port int, account *model.Account) DBClientConfig {
	// Build connection string
	connectionString := fmt.Sprintf("mongodb://%s:%d", ip, port)

	// Add authentication if provided
	args := []string{}
	if account.Account != "" && account.Password != "" {
		// Use --username and --password parameters for MongoDB
		args = append(args, "--username", account.Account, "--password", account.Password)
	}

	// Add the connection string as the last argument
	args = append(args, connectionString)

	return DBClientConfig{
		Command:     "mongosh",
		Args:        args,
		ExitAliases: []string{"exit", "quit"},
	}
}
