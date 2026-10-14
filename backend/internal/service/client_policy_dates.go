package service

import (
	"reflect"
	"slices"
	"time"
)

// 2026-10-14 coder(lq): 日期由服务端维护；旧策略创建时间未知时保留空值，不能使用本次保存时间补造历史。
func stampClientPolicyDates(next, current *ClientAccessConfig, now time.Time) {
	oldClients := make(map[string]ClientDefinition, len(current.Clients))
	newClients := make(map[string]ClientDefinition, len(next.Clients))
	for _, client := range current.Clients {
		oldClients[client.ID] = client
	}
	for _, client := range next.Clients {
		newClients[client.ID] = client
	}
	for id, policy := range next.Groups {
		old, exists := current.Groups[id]
		policy.CreatedAt, policy.UpdatedAt = old.CreatedAt, old.UpdatedAt
		if policy.Mode == "off" {
			next.Groups[id] = policy
			continue
		}
		if !exists || old.Mode == "off" {
			policy.CreatedAt, policy.UpdatedAt = &now, &now
		} else {
			oldIDs, newIDs := slices.Clone(old.ClientIDs), slices.Clone(policy.ClientIDs)
			slices.Sort(oldIDs)
			slices.Sort(newIDs)
			changed := old.Mode != policy.Mode || !slices.Equal(oldIDs, newIDs)
			for _, clientID := range policy.ClientIDs {
				changed = changed || !reflect.DeepEqual(oldClients[clientID], newClients[clientID])
			}
			if changed {
				policy.UpdatedAt = &now
			}
		}
		next.Groups[id] = policy
	}
}
