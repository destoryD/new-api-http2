package model

import "github.com/QuantumNous/new-api/dto"

func isChannelEndpointAllowed(channel *Channel, endpointType string) bool {
	if channel == nil {
		return false
	}
	return dto.IsChannelEndpointAllowed(channel.GetSetting().AllowedEndpointTypes, endpointType)
}

func filterChannelIDsByEndpoint(channelIDs []int, endpointType string) []int {
	endpointType = dto.NormalizeChannelEndpointType(endpointType)
	if endpointType == "" {
		return channelIDs
	}
	filtered := make([]int, 0, len(channelIDs))
	for _, channelID := range channelIDs {
		channel := channelsIDM[channelID]
		if channel == nil {
			filtered = append(filtered, channelID)
			continue
		}
		if isChannelEndpointAllowed(channel, endpointType) {
			filtered = append(filtered, channelID)
		}
	}
	return filtered
}

func filterAbilitiesByEndpoint(abilities []Ability, endpointType string) ([]Ability, error) {
	endpointType = dto.NormalizeChannelEndpointType(endpointType)
	if endpointType == "" || len(abilities) == 0 {
		return abilities, nil
	}
	channelIDs := make([]int, 0, len(abilities))
	for _, ability := range abilities {
		channelIDs = append(channelIDs, ability.ChannelId)
	}
	var channels []Channel
	if err := DB.Select("id", "setting").Where("id IN ?", channelIDs).Find(&channels).Error; err != nil {
		return nil, err
	}
	channelMap := make(map[int]*Channel, len(channels))
	for i := range channels {
		channelMap[channels[i].Id] = &channels[i]
	}
	filtered := make([]Ability, 0, len(abilities))
	for _, ability := range abilities {
		channel := channelMap[ability.ChannelId]
		if channel == nil {
			filtered = append(filtered, ability)
			continue
		}
		if isChannelEndpointAllowed(channel, endpointType) {
			filtered = append(filtered, ability)
		}
	}
	return filtered, nil
}
