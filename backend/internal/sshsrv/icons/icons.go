package icons

// GetDefaultPort returns the default port for each protocol
func GetDefaultPort(protocol string) string {
	switch protocol {
	case "ssh":
		return "22"
	case "rdp":
		return "3389"
	case "vnc":
		return "5900"
	case "mysql":
		return "3306"
	case "redis":
		return "6379"
	case "mongodb":
		return "27017"
	case "postgresql":
		return "5432"
	case "telnet":
		return "23"
	default:
		return ""
	}
}
