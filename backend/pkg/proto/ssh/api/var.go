package api

const (
	// auth
	authUrl = "/public_key/auth"

	// asset
	assetUrl         = "/asset"
	gatewayUrl       = "/gateway"
	commandUrl       = "/command"
	configUrl        = "/config"
	assetTotalUrl    = "/asset/query_by_server"
	assetUpdateState = "/asset/update_by_server"

	// account
	accountUrl = "/account"

	// audit
	sessionUrl    = "/session"
	replayUrl     = "/session/replay"
	replayFileUrl = "/session/replay"
	sessionCmdUrl = "/session/cmd"
)
