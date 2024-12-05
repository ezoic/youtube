package youtube

import (
	"encoding/json"
	"errors"
	"regexp"
)

type YTConfig struct {
	SignatureTimestamp int    `json:"STS"`
	VisitorData        string `json:"VISITOR_DATA"`
}

var ytRegexp = regexp.MustCompile(`ytcfg\.set\s*\(\s*({.+?})\s*\)\s*;`)

func extractYTConfig(html []byte) (YTConfig, error) {
	ytConfig := YTConfig{}

	submatches := ytRegexp.FindSubmatch(html)
	if len(submatches) == 0 {
		return ytConfig, errors.New("could not find yt config")
	}

	ytCfgRaw := submatches[1]

	err := json.Unmarshal(ytCfgRaw, &ytConfig)

	return ytConfig, err
}
