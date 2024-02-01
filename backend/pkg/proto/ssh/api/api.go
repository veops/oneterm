package api

type Authentication interface {
	Authenticate() (token string, err error)
}

type Asset interface {
	Groups() (any, error)
	Lists() (any, error)
}

type Audit interface {
	NewSession(data any) error
}

type Core interface {
	Authentication
	Audit
	Asset
}
