package setting

import (
	"fmt"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
)

const AutoModelDefaultModelsOptionKey = "AutoModelDefaultModels"

var (
	autoModelDefaultModels = []string{}
	autoModelSettingsMu    sync.RWMutex
)

func normalizeAutoModelNames(models []string) []string {
	result := make([]string, 0, len(models))
	seen := make(map[string]struct{}, len(models))
	for _, modelName := range models {
		modelName = strings.TrimSpace(modelName)
		if modelName == "" || strings.EqualFold(modelName, "Auto") {
			continue
		}
		if _, ok := seen[modelName]; ok {
			continue
		}
		seen[modelName] = struct{}{}
		result = append(result, modelName)
	}
	return result
}

func ValidateAutoModelDefaultModels(value string) error {
	var models []string
	if err := common.UnmarshalJsonStr(value, &models); err != nil {
		return fmt.Errorf("无效的 Auto 模型列表: %w", err)
	}
	if len(normalizeAutoModelNames(models)) == 0 {
		return fmt.Errorf("Auto 模型列表至少需要一个模型")
	}
	return nil
}

func UpdateAutoModelDefaultModelsByJSONString(value string) error {
	var models []string
	if err := common.UnmarshalJsonStr(value, &models); err != nil {
		return err
	}
	autoModelSettingsMu.Lock()
	autoModelDefaultModels = normalizeAutoModelNames(models)
	autoModelSettingsMu.Unlock()
	return nil
}

func AutoModelDefaultModels2JSONString() string {
	models := GetAutoModelDefaultModels()
	data, err := common.Marshal(models)
	if err != nil {
		return "[]"
	}
	return string(data)
}

func GetAutoModelDefaultModels() []string {
	autoModelSettingsMu.RLock()
	defer autoModelSettingsMu.RUnlock()
	return append([]string(nil), autoModelDefaultModels...)
}
