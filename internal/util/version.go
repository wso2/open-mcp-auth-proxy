package util

import (
	"time"

	"github.com/wso2/open-mcp-auth-proxy/internal/constants"
)

// This function checks if the given version date is after the spec cutover date
func IsLatestSpec(versionDate time.Time, err error) bool {
	return err == nil && !versionDate.After(constants.SpecCutoverDate)
}

// This function parses a version string into a time.Time
func ParseVersionDate(version string) (time.Time, error) {
	return time.Parse("2006-01-02", version)
}

// This function returns the version string, using the cutover date if empty
func GetVersionWithDefault(version string) string {
	if version == "" {
		return constants.SpecCutoverDate.Format("2006-01-02")
	}
	return version
}
