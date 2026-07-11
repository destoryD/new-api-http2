package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUpdateChannelUsedQuota(t *testing.T) {
	truncateTables(t)

	enabled := true
	disabled := false
	tests := []struct {
		name             string
		disableUsedQuota *bool
		want             int64
	}{
		{name: "records usage by default", want: 42},
		{name: "records usage when explicitly enabled", disableUsedQuota: &disabled, want: 42},
		{name: "skips usage when disabled", disableUsedQuota: &enabled, want: 0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			channel := Channel{
				Name:             test.name,
				Key:              "test-key",
				DisableUsedQuota: test.disableUsedQuota,
			}
			require.NoError(t, DB.Create(&channel).Error)

			UpdateChannelUsedQuota(channel.Id, 42)

			var usedQuota int64
			require.NoError(t, DB.Model(&Channel{}).Where("id = ?", channel.Id).Pluck("used_quota", &usedQuota).Error)
			require.Equal(t, test.want, usedQuota)
		})
	}
}
