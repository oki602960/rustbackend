package models

import (
	"context"
	"testing"
)

func TestAuthsTableLifecycle(t *testing.T) {
	t.Parallel()

	db := openModelsTestDB(t)
	defer db.Close()

	users := NewUsersTable(db)
	auths := NewAuthsTable(db, users)

	user, err := auths.InsertNewAuth(context.Background(), AuthInsertParams{
		Email:           "auth@example.com",
		Password:        "hashed-password",
		Name:            "Auth User",
		ProfileImageURL: "/user.png",
		Role:            "user",
	})
	if err != nil {
		t.Fatal(err)
	}
	if user == nil {
		t.Fatal("expected user from auth insert")
	}

	authenticated, err := auths.AuthenticateUser(context.Background(), "auth@example.com", func(password string) bool {
		return password == "hashed-password"
	})
	if err != nil {
		t.Fatal(err)
	}
	if authenticated == nil || authenticated.ID != user.ID {
		t.Fatal("expected authenticated user")
	}

	ok, err := auths.UpdateUserPasswordByID(context.Background(), user.ID, "new-password")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected password update")
	}

	ok, err = auths.UpdateEmailByID(context.Background(), user.ID, "updated@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected email update")
	}

	deleted, err := auths.DeleteAuthByID(context.Background(), user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !deleted {
		t.Fatal("expected auth delete")
	}
}
