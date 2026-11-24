package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSameGroupSet(t *testing.T) {
	groupIDs := map[string]uint{
		"default": 1,
		"kids":    2,
		"iot":     3,
	}

	assert.True(t, sameGroupSet([]uint{2, 1}, []string{"default", "kids"}, groupIDs))
	assert.True(t, sameGroupSet([]uint{1, 1}, []string{"default", "default"}, groupIDs))
	assert.False(t, sameGroupSet([]uint{1}, []string{"default", "kids"}, groupIDs))
	assert.False(t, sameGroupSet([]uint{1, 2}, []string{"default"}, groupIDs))
	assert.False(t, sameGroupSet([]uint{1}, []string{"unknown"}, groupIDs))
	assert.False(t, sameGroupSet(nil, []string{"default"}, groupIDs))
	assert.False(t, sameGroupSet([]uint{1}, nil, groupIDs))
}
