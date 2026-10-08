package version

// Version information for OneTerm
// Update these values before each release
var (
	// Version is the current version of OneTerm
	Version = "v26.10.1"

	// BuildDate can be set at compile time using ldflags
	// go build -ldflags "-X github.com/veops/oneterm/internal/version.BuildDate=$(date +%Y%m%d)"
	BuildDate = "2026-10-07"

	// GitCommit can be set at compile time using ldflags
	// go build -ldflags "-X github.com/veops/oneterm/internal/version.GitCommit=$(git rev-parse --short HEAD)"
	GitCommit = ""
)

// GetVersion returns the version and build date
func GetVersion() string {
	v := Version
	if BuildDate != "" {
		v += " (" + BuildDate + ")"
	}
	return v
}
