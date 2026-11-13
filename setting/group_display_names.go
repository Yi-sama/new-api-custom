package setting

import (
	"fmt"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
)

const GroupDisplayNamesOptionKey = "GroupDisplayNames"

var (
	groupDisplayNamesMu sync.RWMutex
	groupDisplayNames   = map[string]string{}
)

func parseGroupDisplayNames(value string) (map[string]string, error) {
	var values map[string]any
	if err := common.UnmarshalJsonStr(value, &values); err != nil {
		return nil, fmt.Errorf("group display names must be a JSON object of strings: %w", err)
	}
	if values == nil {
		return nil, fmt.Errorf("group display names must be a JSON object of strings")
	}
	names := make(map[string]string, len(values))
	for group, value := range values {
		name, ok := value.(string)
		if !ok || strings.TrimSpace(group) == "" {
			return nil, fmt.Errorf("group display names must map non-empty group identifiers to strings")
		}
		names[group] = name
	}
	return names, nil
}

func ValidateGroupDisplayNames(value string) error {
	_, err := parseGroupDisplayNames(value)
	return err
}

func UpdateGroupDisplayNamesByJSONString(value string) error {
	names, err := parseGroupDisplayNames(value)
	if err != nil {
		return err
	}
	groupDisplayNamesMu.Lock()
	groupDisplayNames = names
	groupDisplayNamesMu.Unlock()
	return nil
}

func GetGroupDisplayNamesCopy() map[string]string {
	groupDisplayNamesMu.RLock()
	defer groupDisplayNamesMu.RUnlock()
	names := make(map[string]string, len(groupDisplayNames))
	for group, name := range groupDisplayNames {
		names[group] = name
	}
	return names
}

func GroupDisplayNames2JSONString() string {
	groupDisplayNamesMu.RLock()
	defer groupDisplayNamesMu.RUnlock()
	data, err := common.Marshal(groupDisplayNames)
	if err != nil {
		return "{}"
	}
	return string(data)
}
