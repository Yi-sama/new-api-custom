/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
package openai

import (
	"encoding/json"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertOpenAIResponsesRequestRecordsUltraContext(t *testing.T) {
	info := &relaycommon.RelayInfo{}
	request := dto.OpenAIResponsesRequest{
		Model: "gpt-6.1-sol",
		Reasoning: &dto.Reasoning{
			Effort:  "max",
			Context: json.RawMessage(`"all_turns"`),
		},
	}

	converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(nil, info, request)
	require.NoError(t, err)
	assert.Equal(t, "ultra", info.ReasoningEffort)
	assert.Equal(t, "max", converted.(dto.OpenAIResponsesRequest).Reasoning.Effort)
}
