package setting

import (
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
)

const FastModeAllowedGroupsOptionKey = "FastModeAllowedGroups"

var (
	fastModeGroupsMu sync.RWMutex
	fastModeGroups   = []string{}
)

func parseFastModeAllowedGroups(value string) ([]string, error) {
	var groups []string
	if err := common.UnmarshalJsonStr(value, &groups); err != nil {
		return nil, fmt.Errorf("Fast groups must be a JSON string array: %w", err)
	}
	if groups == nil {
		return nil, fmt.Errorf("Fast groups must be a JSON string array")
	}
	for _, group := range groups {
		if strings.TrimSpace(group) == "" || group == "auto" {
			return nil, fmt.Errorf("Fast groups must name concrete, non-empty groups")
		}
	}
	return groups, nil
}

func ValidateFastModeAllowedGroups(value string) error {
	_, err := parseFastModeAllowedGroups(value)
	return err
}

func UpdateFastModeAllowedGroupsByJSONString(value string) error {
	groups, err := parseFastModeAllowedGroups(value)
	if err != nil {
		return err
	}
	fastModeGroupsMu.Lock()
	fastModeGroups = groups
	fastModeGroupsMu.Unlock()
	return nil
}

func FastModeAllowedGroups2JSONString() string {
	fastModeGroupsMu.RLock()
	defer fastModeGroupsMu.RUnlock()
	data, err := common.Marshal(fastModeGroups)
	if err != nil {
		return "[]"
	}
	return string(data)
}

func IsFastModeAllowedForGroup(group string) bool {
	fastModeGroupsMu.RLock()
	defer fastModeGroupsMu.RUnlock()
	return group != "" && group != "auto" && slices.Contains(fastModeGroups, group)
}
