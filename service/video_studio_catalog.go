package service

import (
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
)

type VideoStudioCatalogModel struct {
	ID           string   `json:"id"`
	Capabilities []string `json:"capabilities"`
}

type VideoStudioCatalogGroup struct {
	ID          string                    `json:"id"`
	Description string                    `json:"description"`
	Ratio       float64                   `json:"ratio"`
	Models      []VideoStudioCatalogModel `json:"models"`
}

func GetVideoStudioCatalog(userGroup string) ([]VideoStudioCatalogGroup, error) {
	allowedGroups := GetUserUsableGroups(userGroup)
	modelsByGroup := make(map[string]map[string]map[string]bool)
	for group := range allowedGroups {
		if IsUserSelectableGroup(userGroup, group) {
			modelsByGroup[group] = make(map[string]map[string]bool)
		}
	}
	if len(modelsByGroup) == 0 {
		return make([]VideoStudioCatalogGroup, 0), nil
	}

	metadataEndpoints, metadataAudio, disabledModels, err := videoStudioMetadataEndpoints()
	if err != nil {
		return nil, err
	}
	abiliites, err := model.GetAllEnableAbilityWithChannels()
	if err != nil {
		return nil, err
	}
	channels := make(map[int]*model.Channel)
	for _, ability := range abiliites {
		groupModels, ok := modelsByGroup[ability.Group]
		modelName := strings.TrimSpace(ability.Model)
		if !ok || modelName == "" || disabledModels[modelName] {
			continue
		}
		channel, found := channels[ability.ChannelId]
		if !found {
			var channelErr error
			channel, channelErr = model.GetChannelById(ability.ChannelId, true)
			if channelErr != nil {
				continue
			}
			channels[ability.ChannelId] = channel
		}
		if channel.Status != common.ChannelStatusEnabled || !model.HasAvailableChannelKey(channel) {
			continue
		}
		capabilities := groupModels[modelName]
		if capabilities == nil {
			capabilities = make(map[string]bool)
			groupModels[modelName] = capabilities
		}
		videoStudioAddEndpointCapabilities(capabilities, videoStudioChannelEndpoints(channel, modelName))
		videoStudioAddEndpointCapabilities(capabilities, metadataEndpoints[modelName])
		if metadataAudio[modelName] {
			capabilities["audio"] = true
		}
	}

	groups := make([]VideoStudioCatalogGroup, 0, len(modelsByGroup))
	for group, models := range modelsByGroup {
		catalogModels := make([]VideoStudioCatalogModel, 0, len(models))
		for modelName, capabilities := range models {
			if !capabilities["video"] {
				continue
			}
			catalogModels = append(catalogModels, VideoStudioCatalogModel{
				ID:           modelName,
				Capabilities: videoStudioCapabilityList(capabilities),
			})
		}
		if len(catalogModels) == 0 {
			continue
		}
		sort.Slice(catalogModels, func(i, j int) bool { return catalogModels[i].ID < catalogModels[j].ID })
		groups = append(groups, VideoStudioCatalogGroup{
			ID:          group,
			Description: allowedGroups[group],
			Ratio:       GetUserGroupRatio(userGroup, group),
			Models:      catalogModels,
		})
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].ID < groups[j].ID })
	return groups, nil
}

func IsVideoStudioGroupVideoModel(userGroup, group, modelName string) (bool, error) {
	if !IsUserSelectableGroup(userGroup, group) || strings.TrimSpace(modelName) == "" {
		return false, nil
	}
	groups, err := GetVideoStudioCatalog(userGroup)
	if err != nil {
		return false, err
	}
	for _, entry := range groups {
		if entry.ID != group {
			continue
		}
		for _, entryModel := range entry.Models {
			if entryModel.ID == modelName {
				return true, nil
			}
		}
	}
	return false, nil
}

func videoStudioMetadataEndpoints() (map[string][]constant.EndpointType, map[string]bool, map[string]bool, error) {
	var metadata []model.Model
	if err := model.DB.Find(&metadata).Error; err != nil {
		return nil, nil, nil, err
	}
	endpointsByModel := make(map[string][]constant.EndpointType, len(metadata))
	audioByModel := make(map[string]bool)
	disabledModels := make(map[string]bool)
	for _, entry := range metadata {
		modelName := strings.TrimSpace(entry.ModelName)
		if modelName == "" {
			continue
		}
		if entry.Status != 1 {
			disabledModels[modelName] = true
			continue
		}
		if strings.TrimSpace(entry.Endpoints) == "" {
			continue
		}
		var raw map[string]interface{}
		if err := common.Unmarshal([]byte(entry.Endpoints), &raw); err != nil {
			continue
		}
		for endpoint, value := range raw {
			switch value.(type) {
			case string, map[string]interface{}:
				endpointsByModel[modelName] = append(endpointsByModel[modelName], constant.EndpointType(endpoint))
				if videoStudioMetadataAudioPath(value) {
					audioByModel[modelName] = true
				}
			}
		}
	}
	return endpointsByModel, audioByModel, disabledModels, nil
}

func videoStudioMetadataAudioPath(value interface{}) bool {
	path := ""
	switch typed := value.(type) {
	case string:
		path = typed
	case map[string]interface{}:
		path, _ = typed["path"].(string)
	}
	return strings.TrimSpace(path) == "/v1/audio/speech"
}

func videoStudioChannelEndpoints(channel *model.Channel, modelName string) []constant.EndpointType {
	if channel.Type == constant.ChannelTypeAdvancedCustom {
		return channel.GetOtherSettings().AdvancedCustom.SupportedEndpointTypesForModel(modelName)
	}
	return common.GetEndpointTypesByChannelType(channel.Type, modelName)
}

func videoStudioAddEndpointCapabilities(capabilities map[string]bool, endpoints []constant.EndpointType) {
	for _, endpoint := range endpoints {
		switch endpoint {
		case constant.EndpointTypeOpenAIVideo:
			capabilities["video"] = true
		case constant.EndpointTypeImageGeneration:
			capabilities["image"] = true
		case constant.EndpointTypeOpenAI, constant.EndpointTypeOpenAIResponse, constant.EndpointTypeOpenAIResponseCompact, constant.EndpointTypeAnthropic, constant.EndpointTypeGemini:
			capabilities["text"] = true
		}
	}
}

func videoStudioCapabilityList(capabilities map[string]bool) []string {
	ordered := []string{"video", "image", "text", "audio"}
	result := make([]string, 0, len(ordered))
	for _, capability := range ordered {
		if capabilities[capability] {
			result = append(result, capability)
		}
	}
	return result
}
