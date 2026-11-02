package setting

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGroupDisplayNamesNormalizeOrderedAndLegacyValues(t *testing.T) {
	for _, test := range []struct {
		name, value, expected string
	}{
		{"empty legacy", `{}`, `[]`},
		{"empty array", `[]`, `[]`},
		{"legacy key order", `{"z":" GPT ","a":"Claude"}`, `[{"group":"z","name":"GPT"},{"group":"a","name":"Claude"}]`},
		{"escaped legacy key", `{"a\"b":"GPT"}`, `[{"group":"a\"b","name":"GPT"}]`},
		{"ordered rows", `[{"group":"z","name":" GPT "},{"group":"a","name":"Claude"}]`, `[{"group":"z","name":"GPT"},{"group":"a","name":"Claude"}]`},
		{"unicode length", `[{"group":"z","name":"` + strings.Repeat("模型", 100) + `"}]`, `[{"group":"z","name":"` + strings.Repeat("模型", 100) + `"}]`},
	} {
		t.Run(test.name, func(t *testing.T) {
			value, err := NormalizeGroupDisplayNamesJSON(test.value)
			require.NoError(t, err)
			assert.Equal(t, test.expected, value)
		})
	}
}

func TestGroupDisplayNamesRejectInvalidRows(t *testing.T) {
	for _, value := range []string{
		``, `null`, `1`, `{"default":null}`, `{"default":1}`,
		`[null]`, `[{}]`, `[{"group":"default"}]`, `[{"name":"GPT"}]`,
		`[{"group":" ","name":"GPT"}]`, `[{"group":"default","name":" "}]`,
		`[{"group":"default","name":"GPT"},{"group":"default","name":"Claude"}]`,
		`[{"group":"default","name":"GPT"},{"group":"auto","name":" GPT "}]`,
		`[{"group":"default","name":"` + strings.Repeat("模", 201) + `"}]`,
	} {
		t.Run(value, func(t *testing.T) {
			require.Error(t, ValidateGroupDisplayNames(value))
		})
	}
}
