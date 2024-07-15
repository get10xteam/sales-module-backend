package utils

import (
	"fmt"

	ua "github.com/mileusna/useragent"
)

func ProcessUserAgent(userAgentString string) string {
	userAgentParsed := ua.Parse(userAgentString)
	userAgentParanthesizedPart := userAgentParsed.OS
	if len(userAgentParsed.Device) > 0 {
		userAgentParanthesizedPart = userAgentParsed.OS + " - " + userAgentParsed.Device
	}
	if len(userAgentParanthesizedPart) > 0 {
		uaDesc := fmt.Sprintf("%s (%s)", userAgentParsed.Name, userAgentParanthesizedPart)
		return uaDesc
	} else {
		return userAgentParsed.Name
	}
}
