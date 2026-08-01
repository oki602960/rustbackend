package access_control

import (
	"encoding/json"

	"github.com/xxnuo/open-coreui/backend/open_webui/models"
)

func HasAccess(userID string, permission string, accessGrants []map[string]any, userGroupIDs map[string]struct{}) bool {
	if len(accessGrants) == 0 {
		return false
	}
	for _, grant := range accessGrants {
		if permissionValue, _ := grant["permission"].(string); permissionValue != permission {
			continue
		}
		principalType, _ := grant["principal_type"].(string)
		principalID, _ := grant["principal_id"].(string)
		if principalType == "user" && (principalID == "*" || principalID == userID) {
			return true
		}
		if principalType == "group" {
			if _, ok := userGroupIDs[principalID]; ok {
				return true
			}
		}
	}
	return false
}

func HasConnectionAccess(user *models.User, connection map[string]any, userGroupIDs map[string]struct{}) bool {
	if user == nil {
		return false
	}
	if user.Role == "admin" {
		return true
	}
	config, _ := connection["config"].(map[string]any)
	if config == nil {
		return false
	}
	accessGrants, ok := normalizeAccessGrants(config["access_grants"])
	if !ok {
		return false
	}
	return HasAccess(user.ID, "read", accessGrants, userGroupIDs)
}

func normalizeAccessGrants(value any) ([]map[string]any, bool) {
	if value == nil {
		return []map[string]any{}, true
	}
	body, err := json.Marshal(value)
	if err != nil {
		return nil, false
	}
	var grants []map[string]any
	if err := json.Unmarshal(body, &grants); err != nil {
		return nil, false
	}
	return grants, true
}
