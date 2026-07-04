package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddAmount(t *testing.T) {
	sum, err := AddAmount("10", "20.5")
	require.NoError(t, err)
	assert.Equal(t, "30.50000000", sum)
}

func TestCmpAmount(t *testing.T) {
	cmp, err := CmpAmount("100", "100")
	require.NoError(t, err)
	assert.Equal(t, 0, cmp)

	cmp, err = CmpAmount("99", "100")
	require.NoError(t, err)
	assert.Equal(t, -1, cmp)
}

func TestParseAmountInvalid(t *testing.T) {
	_, err := ParseAmount("-1")
	assert.ErrorIs(t, err, ErrInvalidAmount)
}
