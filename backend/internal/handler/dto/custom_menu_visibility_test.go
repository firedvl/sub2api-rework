package dto

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCustomMenuOpenButtonVisibilitySurvivesPublicProjection(t *testing.T) {
	raw := `[{"id":"visible","label":"Visible","url":"https://example.test","visibility":"user"},{"id":"hidden","label":"Hidden","url":"https://example.test","visibility":"user","hide_open_button":true},{"id":"admin","visibility":"admin","hide_open_button":true}]`
	items := ParseUserVisibleMenuItems(raw)
	require.Len(t, items, 2)
	require.False(t, items[0].HideOpenButton)
	require.True(t, items[1].HideOpenButton)
	encoded, err := json.Marshal(items)
	require.NoError(t, err)
	var decoded []CustomMenuItem
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	require.Equal(t, items, decoded)
}
