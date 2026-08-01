package routers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/xxnuo/open-coreui/backend/open_webui/models"
)

func TestGroupsRouterLifecycle(t *testing.T) {
	t.Parallel()

	db := openRouterTestDB(t)
	defer db.Close()

	users := models.NewUsersTable(db)
	auths := models.NewAuthsTable(db, users)
	groups := models.NewGroupsTable(db)

	authRouter := &AuthsRouter{
		Config: AuthRuntimeConfig{
			WebUIAuth:                true,
			EnableInitialAdminSignup: true,
			EnablePasswordAuth:       true,
			EnableAPIKeys:            true,
			ShowAdminDetails:         true,
			WebUISecretKey:           "secret",
			JWTExpiresIn:             "1h",
			AuthCookieSameSite:       "Lax",
		},
		Users: users,
		Auths: auths,
		Now: func() time.Time {
			return time.Now().UTC()
		},
	}

	groupsRouter := &GroupsRouter{
		Config: GroupsRuntimeConfig{
			WebUISecretKey: "secret",
			EnableAPIKeys:  true,
		},
		Users:  users,
		Groups: groups,
	}

	mux := http.NewServeMux()
	authRouter.Register(mux)
	groupsRouter.Register(mux)

	adminSignupBody, _ := json.Marshal(map[string]any{
		"name":              "Admin",
		"email":             "admin@example.com",
		"password":          "password-123",
		"profile_image_url": "/user.png",
	})
	adminSignupReq := httptest.NewRequest(http.MethodPost, "/api/v1/auths/signup", bytes.NewReader(adminSignupBody))
	adminSignupRes := httptest.NewRecorder()
	mux.ServeHTTP(adminSignupRes, adminSignupReq)
	if adminSignupRes.Code != http.StatusOK {
		t.Fatalf("unexpected admin signup status: %d", adminSignupRes.Code)
	}
	var adminPayload map[string]any
	if err := json.Unmarshal(adminSignupRes.Body.Bytes(), &adminPayload); err != nil {
		t.Fatal(err)
	}
	adminToken, _ := adminPayload["token"].(string)

	addUserBody, _ := json.Marshal(map[string]any{
		"name":              "Member",
		"email":             "member@example.com",
		"password":          "password-123",
		"profile_image_url": "/user.png",
		"role":              "user",
	})
	addUserReq := httptest.NewRequest(http.MethodPost, "/api/v1/auths/add", bytes.NewReader(addUserBody))
	addUserReq.Header.Set("Authorization", "Bearer "+adminToken)
	addUserRes := httptest.NewRecorder()
	mux.ServeHTTP(addUserRes, addUserReq)
	if addUserRes.Code != http.StatusOK {
		t.Fatalf("unexpected add user status: %d", addUserRes.Code)
	}
	var addedUserPayload map[string]any
	if err := json.Unmarshal(addUserRes.Body.Bytes(), &addedUserPayload); err != nil {
		t.Fatal(err)
	}
	memberID, _ := addedUserPayload["id"].(string)
	memberToken, _ := addedUserPayload["token"].(string)

	createGroupBody, _ := json.Marshal(map[string]any{
		"name":        "Group One",
		"description": "desc",
	})
	createGroupReq := httptest.NewRequest(http.MethodPost, "/api/v1/groups/create", bytes.NewReader(createGroupBody))
	createGroupReq.Header.Set("Authorization", "Bearer "+adminToken)
	createGroupRes := httptest.NewRecorder()
	mux.ServeHTTP(createGroupRes, createGroupReq)
	if createGroupRes.Code != http.StatusOK {
		t.Fatalf("unexpected create group status: %d", createGroupRes.Code)
	}
	var groupPayload map[string]any
	if err := json.Unmarshal(createGroupRes.Body.Bytes(), &groupPayload); err != nil {
		t.Fatal(err)
	}
	groupID, _ := groupPayload["id"].(string)

	addUsersBody, _ := json.Marshal(map[string]any{
		"user_ids": []string{memberID},
	})
	addUsersReq := httptest.NewRequest(http.MethodPost, "/api/v1/groups/id/"+groupID+"/users/add", bytes.NewReader(addUsersBody))
	addUsersReq.Header.Set("Authorization", "Bearer "+adminToken)
	addUsersRes := httptest.NewRecorder()
	mux.ServeHTTP(addUsersRes, addUsersReq)
	if addUsersRes.Code != http.StatusOK {
		t.Fatalf("unexpected add users to group status: %d", addUsersRes.Code)
	}

	exportGroupReq := httptest.NewRequest(http.MethodGet, "/api/v1/groups/id/"+groupID+"/export", nil)
	exportGroupReq.Header.Set("Authorization", "Bearer "+adminToken)
	exportGroupRes := httptest.NewRecorder()
	mux.ServeHTTP(exportGroupRes, exportGroupReq)
	if exportGroupRes.Code != http.StatusOK {
		t.Fatalf("unexpected export group status: %d", exportGroupRes.Code)
	}

	memberGroupsReq := httptest.NewRequest(http.MethodGet, "/api/v1/groups/", nil)
	memberGroupsReq.Header.Set("Authorization", "Bearer "+memberToken)
	memberGroupsRes := httptest.NewRecorder()
	mux.ServeHTTP(memberGroupsRes, memberGroupsReq)
	if memberGroupsRes.Code != http.StatusOK {
		t.Fatalf("unexpected member groups status: %d", memberGroupsRes.Code)
	}

	updateGroupBody, _ := json.Marshal(map[string]any{
		"name":        "Group One Updated",
		"description": "desc2",
	})
	updateGroupReq := httptest.NewRequest(http.MethodPost, "/api/v1/groups/id/"+groupID+"/update", bytes.NewReader(updateGroupBody))
	updateGroupReq.Header.Set("Authorization", "Bearer "+adminToken)
	updateGroupRes := httptest.NewRecorder()
	mux.ServeHTTP(updateGroupRes, updateGroupReq)
	if updateGroupRes.Code != http.StatusOK {
		t.Fatalf("unexpected update group status: %d", updateGroupRes.Code)
	}

	groupUsersReq := httptest.NewRequest(http.MethodPost, "/api/v1/groups/id/"+groupID+"/users", nil)
	groupUsersReq.Header.Set("Authorization", "Bearer "+adminToken)
	groupUsersRes := httptest.NewRecorder()
	mux.ServeHTTP(groupUsersRes, groupUsersReq)
	if groupUsersRes.Code != http.StatusOK {
		t.Fatalf("unexpected group users status: %d", groupUsersRes.Code)
	}

	removeUsersBody, _ := json.Marshal(map[string]any{
		"user_ids": []string{memberID},
	})
	removeUsersReq := httptest.NewRequest(http.MethodPost, "/api/v1/groups/id/"+groupID+"/users/remove", bytes.NewReader(removeUsersBody))
	removeUsersReq.Header.Set("Authorization", "Bearer "+adminToken)
	removeUsersRes := httptest.NewRecorder()
	mux.ServeHTTP(removeUsersRes, removeUsersReq)
	if removeUsersRes.Code != http.StatusOK {
		t.Fatalf("unexpected remove users status: %d", removeUsersRes.Code)
	}

	deleteGroupReq := httptest.NewRequest(http.MethodDelete, "/api/v1/groups/id/"+groupID, nil)
	deleteGroupReq.Header.Set("Authorization", "Bearer "+adminToken)
	deleteGroupRes := httptest.NewRecorder()
	mux.ServeHTTP(deleteGroupRes, deleteGroupReq)
	if deleteGroupRes.Code != http.StatusOK {
		t.Fatalf("unexpected delete group status: %d", deleteGroupRes.Code)
	}
}
