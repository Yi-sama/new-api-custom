package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFilterAutoModelNamesPreservesConfiguredOrder(t *testing.T) {
	actual := FilterAutoModelNames(
		[]string{" model-b ", "model-a", "model-b", "Auto", "missing"},
		[]string{"model-a", "model-b"},
	)

	assert.Equal(t, []string{"model-b", "model-a"}, actual)
	assert.NotNil(t, FilterAutoModelNames(nil, nil))
}

func TestNormalizeAutoModelStrategyDefaultsToPrice(t *testing.T) {
	assert.Equal(t, AutoModelStrategyOrder, NormalizeAutoModelStrategy(AutoModelStrategyOrder))
	assert.Equal(t, AutoModelStrategyPrice, NormalizeAutoModelStrategy("unknown"))
}
