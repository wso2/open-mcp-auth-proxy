package constants

import "time"

// Package constant provides constants for the MCP Auth Proxy

const (
	ASGARDEO_BASE_URL = "https://api.asgardeo.io/t/"
)

// MCP specification version cutover date
var SpecCutoverDate = time.Date(2025, 3, 26, 0, 0, 0, 0, time.UTC)

const TimeLayout = "2006-01-02"
