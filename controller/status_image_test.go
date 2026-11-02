package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatusImageOverviewPreservesLegacyAndIndependentFilterSelections(t *testing.T) {
	t.Setenv("SUB2API_BASE_URL", "")
	t.Setenv("SUB2API_ADMIN_API_KEY", "")
	common.OptionMapRWMutex.Lock()
	previous := common.OptionMap
	common.OptionMap = map[string]string{}
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previous
		common.OptionMapRWMutex.Unlock()
	})

	legacy, selected, blank := "Legacy pool", "Channel pool", ""
	emptyModels := []model.StatusImageModelSelection{}
	selectedModels := []model.StatusImageModelSelection{{Group: "A", Model: "gpt-5"}}
	for _, tc := range []struct {
		name    string
		stored  bool
		channel *string
		sub     *string
		models  *[]model.StatusImageModelSelection
	}{
		{name: "defaults", channel: &blank, sub: &blank},
		{name: "legacy", stored: true},
		{name: "independent", stored: true, channel: &selected, sub: &blank},
		{name: "no models", stored: true, models: &emptyModels},
		{name: "exact models", stored: true, models: &selectedModels},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := model.DefaultStatusImageConfig()
			cfg.GroupFilter = &legacy
			cfg.ChannelGroupFilter, cfg.SubGroupFilter = tc.channel, tc.sub
			cfg.ChannelModelFilter = tc.models
			raw, err := common.Marshal(cfg)
			require.NoError(t, err)
			common.OptionMapRWMutex.Lock()
			delete(common.OptionMap, model.StatusImageConfigOptionKey)
			if tc.stored {
				common.OptionMap[model.StatusImageConfigOptionKey] = string(raw)
			}
			common.OptionMapRWMutex.Unlock()
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodGet, "/api/status-images/overview?kind=sub", nil)

			GetStatusImageOverview(ctx)

			require.Equal(t, http.StatusOK, recorder.Code)
			var response struct {
				Success bool `json:"success"`
				Data    struct {
					Config model.StatusImageConfig `json:"config"`
				} `json:"data"`
			}
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
			require.True(t, response.Success, recorder.Body.String())
			assert.Equal(t, tc.channel, response.Data.Config.ChannelGroupFilter)
			assert.Equal(t, tc.sub, response.Data.Config.SubGroupFilter)
			assert.Equal(t, tc.models, response.Data.Config.ChannelModelFilter)
			if tc.stored {
				assert.Equal(t, &legacy, response.Data.Config.GroupFilter)
			}
		})
	}
}

func TestStatusImageIncludesUnmonitoredChannelGroupsWithoutChangingMonitoring(t *testing.T) {
	previous := ratio_setting.GroupRatio2JSONString()
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"image-test-A":1,"image-test-B":1}`))
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(previous))
	})
	channels := []*model.Channel{
		{Status: common.ChannelStatusEnabled, Group: "image-test-A,image-test-B", Models: "model-A"},
		{Status: common.ChannelStatusManuallyDisabled, Group: "disabled-image-test", Models: "model-B"},
	}
	groups := statusImageGroups(channels)
	var a, b *model.ChannelStatusGroupConfig
	for i := range groups {
		switch groups[i].Group {
		case "image-test-A":
			a = &groups[i]
		case "image-test-B":
			b = &groups[i]
		case "disabled-image-test":
			t.Fatal("disabled-only channel must not become a live status group")
		}
	}
	require.NotNil(t, a)
	require.NotNil(t, b)
	assert.Equal(t, []string{"model-A"}, a.Models)
	assert.Equal(t, []string{"model-A"}, b.Models)
	assert.True(t, a.Enabled)
}

func TestStatusImageStopsListingDeletedGroupsEvenWhenChannelsRetainTheirMembership(t *testing.T) {
	previous := ratio_setting.GroupRatio2JSONString()
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"Current pool":1,"Deleted pool":1}`))
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(previous))
	})
	channels := []*model.Channel{
		{Status: common.ChannelStatusEnabled, Group: "Current pool,Deleted pool", Models: "gpt-test"},
	}
	require.Len(t, statusImageGroups(channels), 2)

	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"Current pool":1,"Empty pool":1}`))

	groups := statusImageGroups(channels)
	require.Len(t, groups, 2)
	assert.Equal(t, "Current pool", groups[0].Group)
	assert.Equal(t, []string{"gpt-test"}, groups[0].Models)
	assert.Equal(t, "Empty pool", groups[1].Group)
	assert.Empty(t, groups[1].Models)
	assert.Equal(t, "Current pool,Deleted pool", channels[0].Group)
}
