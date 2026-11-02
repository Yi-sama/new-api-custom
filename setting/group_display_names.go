package setting

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/tidwall/gjson"
)

const GroupDisplayNamesOptionKey = "GroupDisplayNames"

var (
	groupDisplayNamesMu sync.RWMutex
	groupDisplayNames   = []GroupDisplayName{}
)

type GroupDisplayName struct {
	Group string `json:"group"`
	Name  string `json:"name"`
}

func parseGroupDisplayNames(value string) ([]GroupDisplayName, error) {
	var raw json.RawMessage
	if err := common.UnmarshalJsonStr(value, &raw); err != nil {
		return nil, fmt.Errorf("invalid group display names: %w", err)
	}
	names := make([]GroupDisplayName, 0)
	switch common.GetJsonType(raw) {
	case "array":
		if err := common.Unmarshal(raw, &names); err != nil {
			return nil, fmt.Errorf("group display names must contain group and name strings: %w", err)
		}
	case "object":
		// Legacy objects retain their JSON key order instead of Go map iteration order.
		var invalid bool
		gjson.ParseBytes(raw).ForEach(func(group, name gjson.Result) bool {
			if name.Type != gjson.String {
				invalid = true
				return false
			}
			names = append(names, GroupDisplayName{Group: group.String(), Name: name.String()})
			return true
		})
		if invalid {
			return nil, fmt.Errorf("group display names must map group identifiers to strings")
		}
	default:
		return nil, fmt.Errorf("group display names must be an array or a legacy JSON object")
	}
	groups, labels := make(map[string]bool), make(map[string]bool)
	for index := range names {
		row := &names[index]
		row.Name = strings.TrimSpace(row.Name)
		if strings.TrimSpace(row.Group) == "" || row.Name == "" {
			return nil, fmt.Errorf("row %d: group and display name are required", index+1)
		}
		if utf8.RuneCountInString(row.Name) > 200 {
			return nil, fmt.Errorf("row %d: display name must be at most 200 characters", index+1)
		}
		if groups[row.Group] {
			return nil, fmt.Errorf("row %d: each group can only appear once", index+1)
		}
		if labels[row.Name] {
			return nil, fmt.Errorf("row %d: display names must be unique", index+1)
		}
		groups[row.Group], labels[row.Name] = true, true
	}
	return names, nil
}

func ValidateGroupDisplayNames(value string) error {
	_, err := parseGroupDisplayNames(value)
	return err
}

func NormalizeGroupDisplayNamesJSON(value string) (string, error) {
	names, err := parseGroupDisplayNames(value)
	if err != nil {
		return "", err
	}
	data, err := common.Marshal(names)
	return string(data), err
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

func GetGroupDisplayNamesCopy() []GroupDisplayName {
	groupDisplayNamesMu.RLock()
	defer groupDisplayNamesMu.RUnlock()
	return append([]GroupDisplayName{}, groupDisplayNames...)
}

func GroupDisplayNames2JSONString() string {
	groupDisplayNamesMu.RLock()
	defer groupDisplayNamesMu.RUnlock()
	data, err := common.Marshal(groupDisplayNames)
	if err != nil {
		return "[]"
	}
	return string(data)
}
