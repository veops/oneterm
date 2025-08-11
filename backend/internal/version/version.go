package version

// Version information for OneTerm
// Update these values before each release
const (
	// Version is the current version of OneTerm
	Version = "v25.8.1"

	// BuildDate can be set at compile time using ldflags
	// go build -ldflags "-X github.com/veops/oneterm/internal/version.BuildDate=$(date +%Y%m%d)"
	BuildDate = ""

	// GitCommit can be set at compile time using ldflags
	// go build -ldflags "-X github.com/veops/oneterm/internal/version.GitCommit=$(git rev-parse --short HEAD)"
	GitCommit = ""
)

// GetVersion returns the full version string
func GetVersion() string {
	v := Version
	if GitCommit != "" {
		v += "-" + GitCommit
	}
	if BuildDate != "" {
		v += " (" + BuildDate + ")"
	}
	return v
}
