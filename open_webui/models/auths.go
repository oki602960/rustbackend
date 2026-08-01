package models

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	dbinternal "github.com/xxnuo/open-coreui/backend/open_webui/internal"
)

type Auth struct {
	ID       string
	Email    string
	Password string
	Active   bool
}

type AuthInsertParams struct {
	Email           string
	Password        string
	Name            string
	ProfileImageURL string
	Role            string
}

type AuthsTable struct {
	db    *dbinternal.Handle
	users *UsersTable
}

func NewAuthsTable(db *dbinternal.Handle, users *UsersTable) *AuthsTable {
	return &AuthsTable{
		db:    db,
		users: users,
	}
}

func (t *AuthsTable) InsertNewAuth(ctx context.Context, params AuthInsertParams) (*User, error) {
	tx, err := t.db.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	id := uuid.NewString()
	role := params.Role
	if role == "" {
		role = "pending"
	}
	now := time.Now().Unix()

	_, err = tx.ExecContext(
		ctx,
		rebindPlaceholders(`INSERT INTO auth (id, email, password, active) VALUES (?, ?, ?, ?)`, t.db.Dialect),
		id,
		params.Email,
		params.Password,
		true,
	)
	if err != nil {
		return nil, err
	}

	profileImageURL := params.ProfileImageURL
	if profileImageURL == "" {
		profileImageURL = "/user.png"
	}

	_, err = tx.ExecContext(
		ctx,
		rebindPlaceholders(`INSERT INTO "user" (id, name, email, role, profile_image_url, last_active_at, updated_at, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, t.db.Dialect),
		id,
		params.Name,
		params.Email,
		role,
		profileImageURL,
		now,
		now,
		now,
	)
	if err != nil {
		return nil, err
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}

	return t.users.GetUserByID(ctx, id)
}

func (t *AuthsTable) AuthenticateUser(ctx context.Context, email string, verifyPassword func(string) bool) (*User, error) {
	user, err := t.users.GetUserByEmail(ctx, email)
	if err != nil || user == nil {
		return user, err
	}

	auth, err := t.getAuthByID(ctx, user.ID)
	if err != nil || auth == nil || !auth.Active {
		return nil, err
	}
	if !verifyPassword(auth.Password) {
		return nil, nil
	}
	return user, nil
}

func (t *AuthsTable) AuthenticateUserByAPIKey(ctx context.Context, apiKey string) (*User, error) {
	if apiKey == "" {
		return nil, nil
	}
	return t.users.GetUserByAPIKey(ctx, apiKey)
}

func (t *AuthsTable) AuthenticateUserByEmail(ctx context.Context, email string) (*User, error) {
	user, err := t.users.GetUserByEmail(ctx, email)
	if err != nil || user == nil {
		return user, err
	}

	auth, err := t.getAuthByID(ctx, user.ID)
	if err != nil || auth == nil || !auth.Active {
		return nil, err
	}
	return user, nil
}

func (t *AuthsTable) UpdateUserPasswordByID(ctx context.Context, id string, newPassword string) (bool, error) {
	result, err := t.db.DB.ExecContext(ctx, rebindPlaceholders(`UPDATE auth SET password = ? WHERE id = ?`, t.db.Dialect), newPassword, id)
	if err != nil {
		return false, err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return rowsAffected == 1, nil
}

func (t *AuthsTable) UpdateEmailByID(ctx context.Context, id string, email string) (bool, error) {
	tx, err := t.db.DB.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	authResult, err := tx.ExecContext(ctx, rebindPlaceholders(`UPDATE auth SET email = ? WHERE id = ?`, t.db.Dialect), email, id)
	if err != nil {
		return false, err
	}
	userResult, err := tx.ExecContext(ctx, rebindPlaceholders(`UPDATE "user" SET email = ? WHERE id = ?`, t.db.Dialect), email, id)
	if err != nil {
		return false, err
	}

	if err = tx.Commit(); err != nil {
		return false, err
	}

	authRows, err := authResult.RowsAffected()
	if err != nil {
		return false, err
	}
	userRows, err := userResult.RowsAffected()
	if err != nil {
		return false, err
	}
	return authRows == 1 && userRows == 1, nil
}

func (t *AuthsTable) DeleteAuthByID(ctx context.Context, id string) (bool, error) {
	tx, err := t.db.DB.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	_, err = tx.ExecContext(ctx, rebindPlaceholders(`DELETE FROM "user" WHERE id = ?`, t.db.Dialect), id)
	if err != nil {
		return false, err
	}
	result, err := tx.ExecContext(ctx, rebindPlaceholders(`DELETE FROM auth WHERE id = ?`, t.db.Dialect), id)
	if err != nil {
		return false, err
	}
	if err = tx.Commit(); err != nil {
		return false, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return rowsAffected > 0, nil
}

func (t *AuthsTable) getAuthByID(ctx context.Context, id string) (*Auth, error) {
	row := t.db.DB.QueryRowContext(ctx, rebindPlaceholders(`SELECT id, email, password, active FROM auth WHERE id = ? LIMIT 1`, t.db.Dialect), id)

	var auth Auth
	err := row.Scan(&auth.ID, &auth.Email, &auth.Password, &auth.Active)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &auth, nil
}
