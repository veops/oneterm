package db

import (
	"github.com/veops/oneterm/internal/model"
	gsession "github.com/veops/oneterm/internal/session"
)

// DBClientConfig holds the configuration for a database client
type DBClientConfig struct {
	Command     string
	Args        []string
	ExitAliases []string
}

// ConnectDB connects to a database with the given session, asset, account, and gateway
func ConnectDB(sess *gsession.Session, asset *model.Asset, account *model.Account, gateway *model.Gateway) error {
	return connectDB(sess, asset, account, gateway)
}
