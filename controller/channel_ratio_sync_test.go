package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/stretchr/testify/require"
)

func TestFetchChannelRatioSyncGroupsUsesPricingEndpoint(t *testing.T) {
	var requestedPath string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestedPath = request.URL.Path
		_, _ = writer.Write([]byte(`{"success":true,"group_ratio":{"default":1,"vip":0.8}}`))
	}))
	defer server.Close()

	baseURL := server.URL + "/"
	channel := &model.Channel{BaseURL: &baseURL}
	groups, err := fetchChannelRatioSyncGroups(context.Background(), channel)

	require.NoError(t, err)
	require.Equal(t, "/api/pricing", requestedPath)
	require.Equal(t, []channelRatioSyncGroup{
		{Name: "default", Ratio: 1},
		{Name: "vip", Ratio: 0.8},
	}, groups)
}

func TestFetchChannelRatioSyncGroupsKeepsExplicitPricingEndpoint(t *testing.T) {
	var requestedPath string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestedPath = request.URL.Path
		_, _ = writer.Write([]byte(`{"success":true,"group_ratio":{"default":1}}`))
	}))
	defer server.Close()

	sourceURL := server.URL + "/api/pricing/"
	channel := &model.Channel{}
	channel.SetOtherSettings(channelOtherSettingsWithRatioSource(sourceURL))

	_, err := fetchChannelRatioSyncGroups(context.Background(), channel)

	require.NoError(t, err)
	require.Equal(t, "/api/pricing", requestedPath)
}

func channelOtherSettingsWithRatioSource(sourceURL string) dto.ChannelOtherSettings {
	return dto.ChannelOtherSettings{ChannelRatioSyncSourceURL: sourceURL}
}
