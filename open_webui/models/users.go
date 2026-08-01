package models

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	dbinternal "github.com/xxnuo/open-coreui/backend/open_webui/internal"
)

type User struct {
	ID                    string
	Email                 string
	Username              string
	Role                  string
	Name                  string
	ProfileImageURL       string
	ProfileBannerImageURL string
	Bio                   string
	Gender                string
	DateOfBirth           string
	Timezone              string
	PresenceState         string
	StatusEmoji           string
	StatusMessage         string
	StatusExpiresAt       *int64
	Settings              map[string]any
	Info                  map[string]any
	OAuth                 map[string]any
	SCIM                  map[string]any
	LastActiveAt          int64
	UpdatedAt             int64
	CreatedAt             int64
}

type UserInsertParams struct {
	ID              string
	Email           string
	Username        string
	Role            string
	Name            string
	ProfileImageURL string
	Settings        map[string]any
	Info            map[string]any
	OAuth           map[string]any
	SCIM            map[string]any
}

type UserUpdateParams struct {
	Email                 *string
	Username              *string
	Role                  *string
	Name                  *string
	ProfileImageURL       *string
	ProfileBannerImageURL *string
	Bio                   *string
	Gender                *string
	DateOfBirth           *string
	Timezone              *string
	PresenceState         *string
	StatusEmoji           *string
	StatusMessage         *string
	StatusExpiresAt       *int64
}

type UsersTable struct {
	db *dbinternal.Handle
}

type UserListOptions struct {
	Query     string
	OrderBy   string
	Direction string
	Skip      int
	Limit     int
}

func NewUsersTable(db *dbinternal.Handle) *UsersTable {
	return &UsersTable{db: db}
}

func (t *UsersTable) InsertNewUser(ctx context.Context, params UserInsertParams) (*User, error) {
	now := time.Now().Unix()
	role := params.Role
	if role == "" {
		role = "pending"
	}
	profileImageURL := params.ProfileImageURL
	if strings.TrimSpace(profileImageURL) == "" {
		profileImageURL = "/user.png"
	}

	settingsJSON, err := marshalJSONMap(params.Settings)
	if err != nil {
		return nil, err
	}
	infoJSON, err := marshalJSONMap(params.Info)
	if err != nil {
		return nil, err
	}
	oauthJSON, err := marshalJSONMap(params.OAuth)
	if err != nil {
		return nil, err
	}
	scimJSON, err := marshalJSONMap(params.SCIM)
	if err != nil {
		return nil, err
	}

	_, err = t.db.DB.ExecContext(
		ctx,
		rebindPlaceholders(`INSERT INTO "user" (id, name, email, username, role, profile_image_url, last_active_at, updated_at, created_at, settings, info, oauth, scim) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, t.db.Dialect),
		params.ID,
		params.Name,
		params.Email,
		params.Username,
		role,
		profileImageURL,
		now,
		now,
		now,
		settingsJSON,
		infoJSON,
		oauthJSON,
		scimJSON,
	)
	if err != nil {
		return nil, err
	}
	return t.GetUserByID(ctx, params.ID)
}

func (t *UsersTable) GetUserByID(ctx context.Context, id string) (*User, error) {
	row := t.db.DB.QueryRowContext(
		ctx,
		rebindPlaceholders(`SELECT id, email, username, role, name, profile_image_url, profile_banner_image_url, bio, gender, CAST(date_of_birth AS TEXT), timezone, presence_state, status_emoji, status_message, status_expires_at, settings, info, oauth, scim, last_active_at, updated_at, created_at FROM "user" WHERE id = ? LIMIT 1`, t.db.Dialect),
		id,
	)
	return scanUser(row)
}

func (t *UsersTable) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	row := t.db.DB.QueryRowContext(
		ctx,
		rebindPlaceholders(`SELECT id, email, username, role, name, profile_image_url, profile_banner_image_url, bio, gender, CAST(date_of_birth AS TEXT), timezone, presence_state, status_emoji, status_message, status_expires_at, settings, info, oauth, scim, last_active_at, updated_at, created_at FROM "user" WHERE LOWER(email) = LOWER(?) LIMIT 1`, t.db.Dialect),
		email,
	)
	return scanUser(row)
}

func (t *UsersTable) GetUserByAPIKey(ctx context.Context, apiKey string) (*User, error) {
	exists, err := t.db.TableExists(ctx, "api_key")
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil
	}

	row := t.db.DB.QueryRowContext(
		ctx,
		rebindPlaceholders(`SELECT u.id, u.email, u.username, u.role, u.name, u.profile_image_url, u.profile_banner_image_url, u.bio, u.gender, CAST(u.date_of_birth AS TEXT), u.timezone, u.presence_state, u.status_emoji, u.status_message, u.status_expires_at, u.settings, u.info, u.oauth, u.scim, u.last_active_at, u.updated_at, u.created_at
		FROM "user" u
		JOIN api_key a ON u.id = a.user_id
		WHERE a.key = ?
		LIMIT 1`, t.db.Dialect),
		apiKey,
	)
	return scanUser(row)
}

func (t *UsersTable) UpdateUserByID(ctx context.Context, id string, updates UserUpdateParams) (*User, error) {
	assignments := make([]string, 0, 12)
	args := make([]any, 0, 13)

	if updates.Email != nil {
		assignments = append(assignments, "email = ?")
		args = append(args, *updates.Email)
	}
	if updates.Username != nil {
		assignments = append(assignments, "username = ?")
		args = append(args, *updates.Username)
	}
	if updates.Role != nil {
		assignments = append(assignments, "role = ?")
		args = append(args, *updates.Role)
	}
	if updates.Name != nil {
		assignments = append(assignments, "name = ?")
		args = append(args, *updates.Name)
	}
	if updates.ProfileImageURL != nil {
		assignments = append(assignments, "profile_image_url = ?")
		args = append(args, *updates.ProfileImageURL)
	}
	if updates.ProfileBannerImageURL != nil {
		assignments = append(assignments, "profile_banner_image_url = ?")
		args = append(args, *updates.ProfileBannerImageURL)
	}
	if updates.Bio != nil {
		assignments = append(assignments, "bio = ?")
		args = append(args, *updates.Bio)
	}
	if updates.Gender != nil {
		assignments = append(assignments, "gender = ?")
		args = append(args, *updates.Gender)
	}
	if updates.DateOfBirth != nil {
		assignments = append(assignments, "date_of_birth = ?")
		args = append(args, *updates.DateOfBirth)
	}
	if updates.Timezone != nil {
		assignments = append(assignments, "timezone = ?")
		args = append(args, *updates.Timezone)
	}
	if updates.PresenceState != nil {
		assignments = append(assignments, "presence_state = ?")
		args = append(args, *updates.PresenceState)
	}
	if updates.StatusEmoji != nil {
		assignments = append(assignments, "status_emoji = ?")
		args = append(args, *updates.StatusEmoji)
	}
	if updates.StatusMessage != nil {
		assignments = append(assignments, "status_message = ?")
		args = append(args, *updates.StatusMessage)
	}
	if updates.StatusExpiresAt != nil {
		assignments = append(assignments, "status_expires_at = ?")
		args = append(args, *updates.StatusExpiresAt)
	}
	if len(assignments) == 0 {
		return t.GetUserByID(ctx, id)
	}

	assignments = append(assignments, "updated_at = ?")
	args = append(args, time.Now().Unix(), id)

	query := rebindPlaceholders(`UPDATE "user" SET `+strings.Join(assignments, ", ")+` WHERE id = ?`, t.db.Dialect)
	result, err := t.db.DB.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if rowsAffected == 0 {
		return nil, nil
	}
	return t.GetUserByID(ctx, id)
}

func (t *UsersTable) DeleteUserByID(ctx context.Context, id string) (bool, error) {
	result, err := t.db.DB.ExecContext(ctx, rebindPlaceholders(`DELETE FROM "user" WHERE id = ?`, t.db.Dialect), id)
	if err != nil {
		return false, err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return rowsAffected > 0, nil
}

func (t *UsersTable) GetUserAPIKeyByID(ctx context.Context, id string) (string, error) {
	exists, err := t.db.TableExists(ctx, "api_key")
	if err != nil {
		return "", err
	}
	if !exists {
		return "", nil
	}

	row := t.db.DB.QueryRowContext(
		ctx,
		rebindPlaceholders(`SELECT key FROM api_key WHERE user_id = ? LIMIT 1`, t.db.Dialect),
		id,
	)
	var key string
	err = row.Scan(&key)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return key, nil
}

func (t *UsersTable) UpdateUserAPIKeyByID(ctx context.Context, id string, apiKey string) (bool, error) {
	tx, err := t.db.DB.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if _, err = tx.ExecContext(ctx, rebindPlaceholders(`DELETE FROM api_key WHERE user_id = ?`, t.db.Dialect), id); err != nil {
		return false, err
	}

	now := time.Now().Unix()
	if _, err = tx.ExecContext(
		ctx,
		rebindPlaceholders(`INSERT INTO api_key (id, user_id, key, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`, t.db.Dialect),
		"key_"+id,
		id,
		apiKey,
		now,
		now,
	); err != nil {
		return false, err
	}

	if err = tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

func (t *UsersTable) DeleteUserAPIKeyByID(ctx context.Context, id string) (bool, error) {
	result, err := t.db.DB.ExecContext(
		ctx,
		rebindPlaceholders(`DELETE FROM api_key WHERE user_id = ?`, t.db.Dialect),
		id,
	)
	if err != nil {
		return false, err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return rowsAffected >= 0, nil
}

func (t *UsersTable) GetValidUserIDs(ctx context.Context, userIDs []string) ([]string, error) {
	if len(userIDs) == 0 {
		return []string{}, nil
	}

	query := `SELECT id FROM "user" WHERE id IN (` + placeholdersForUsers(len(userIDs), t.db.Dialect) + `)`
	args := make([]any, 0, len(userIDs))
	for _, userID := range userIDs {
		args = append(args, userID)
	}
	rows, err := t.db.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		result = append(result, id)
	}
	return result, rows.Err()
}

func (t *UsersTable) GetUsersByGroupID(ctx context.Context, groupID string) ([]User, error) {
	rows, err := t.db.DB.QueryContext(
		ctx,
		rebindPlaceholders(`SELECT u.id, u.email, u.username, u.role, u.name, u.profile_image_url, u.profile_banner_image_url, u.bio, u.gender, CAST(u.date_of_birth AS TEXT), u.timezone, u.presence_state, u.status_emoji, u.status_message, u.status_expires_at, u.settings, u.info, u.oauth, u.scim, u.last_active_at, u.updated_at, u.created_at
		FROM "user" u
		JOIN group_member gm ON gm.user_id = u.id
		WHERE gm.group_id = ?
		ORDER BY u.name ASC`, t.db.Dialect),
		groupID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		user, err := scanUserRows(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, *user)
	}
	return users, rows.Err()
}

func (t *UsersTable) UpdateUserSettingsByID(ctx context.Context, id string, updated map[string]any) (*User, error) {
	user, err := t.GetUserByID(ctx, id)
	if err != nil || user == nil {
		return user, err
	}

	settings := cloneJSONMap(user.Settings)
	if settings == nil {
		settings = map[string]any{}
	}
	for key, value := range updated {
		settings[key] = value
	}

	settingsJSON, err := marshalJSONMap(settings)
	if err != nil {
		return nil, err
	}
	_, err = t.db.DB.ExecContext(
		ctx,
		rebindPlaceholders(`UPDATE "user" SET settings = ?, updated_at = ? WHERE id = ?`, t.db.Dialect),
		settingsJSON,
		time.Now().Unix(),
		id,
	)
	if err != nil {
		return nil, err
	}
	return t.GetUserByID(ctx, id)
}

func (t *UsersTable) UpdateUserInfoByID(ctx context.Context, id string, updated map[string]any) (*User, error) {
	user, err := t.GetUserByID(ctx, id)
	if err != nil || user == nil {
		return user, err
	}

	info := cloneJSONMap(user.Info)
	if info == nil {
		info = map[string]any{}
	}
	for key, value := range updated {
		info[key] = value
	}

	infoJSON, err := marshalJSONMap(info)
	if err != nil {
		return nil, err
	}
	_, err = t.db.DB.ExecContext(
		ctx,
		rebindPlaceholders(`UPDATE "user" SET info = ?, updated_at = ? WHERE id = ?`, t.db.Dialect),
		infoJSON,
		time.Now().Unix(),
		id,
	)
	if err != nil {
		return nil, err
	}
	return t.GetUserByID(ctx, id)
}

func (t *UsersTable) CountUsers(ctx context.Context) (int, error) {
	row := t.db.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM "user"`)
	var count int
	if err := row.Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (t *UsersTable) HasUsers(ctx context.Context) (bool, error) {
	count, err := t.CountUsers(ctx)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (t *UsersTable) GetFirstUser(ctx context.Context) (*User, error) {
	row := t.db.DB.QueryRowContext(
		ctx,
		`SELECT id, email, username, role, name, profile_image_url, profile_banner_image_url, bio, gender, CAST(date_of_birth AS TEXT), timezone, presence_state, status_emoji, status_message, status_expires_at, settings, info, oauth, scim, last_active_at, updated_at, created_at FROM "user" ORDER BY created_at ASC LIMIT 1`,
	)
	return scanUser(row)
}

func IsActive(user *User) bool {
	if user == nil {
		return false
	}
	if user.LastActiveAt == 0 {
		return false
	}
	return user.LastActiveAt >= time.Now().Unix()-180
}

func (t *UsersTable) IsUserActive(ctx context.Context, userID string) (bool, error) {
	user, err := t.GetUserByID(ctx, userID)
	if err != nil || user == nil {
		return false, err
	}
	return IsActive(user), nil
}

func (t *UsersTable) GetUsers(ctx context.Context, options UserListOptions) ([]User, int, error) {
	query := `SELECT id, email, username, role, name, profile_image_url, profile_banner_image_url, bio, gender, CAST(date_of_birth AS TEXT), timezone, presence_state, status_emoji, status_message, status_expires_at, settings, info, oauth, scim, last_active_at, updated_at, created_at FROM "user"`
	countQuery := `SELECT COUNT(*) FROM "user"`
	clauses := []string{}
	args := []any{}

	if strings.TrimSpace(options.Query) != "" {
		clauses = append(clauses, `(LOWER(name) LIKE LOWER(?) OR LOWER(email) LIKE LOWER(?))`)
		queryValue := "%" + options.Query + "%"
		args = append(args, queryValue, queryValue)
	}

	if len(clauses) > 0 {
		where := " WHERE " + strings.Join(clauses, " AND ")
		query += where
		countQuery += where
	}

	orderBy := "created_at"
	switch options.OrderBy {
	case "name", "email", "created_at", "last_active_at", "updated_at", "role":
		orderBy = options.OrderBy
	}
	direction := "DESC"
	if strings.EqualFold(options.Direction, "asc") {
		direction = "ASC"
	}
	query += " ORDER BY " + orderBy + " " + direction

	var total int
	if err := t.db.DB.QueryRowContext(ctx, rebindPlaceholders(countQuery, t.db.Dialect), args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	pagedArgs := append([]any{}, args...)
	if options.Limit > 0 {
		query += " LIMIT ?"
		pagedArgs = append(pagedArgs, options.Limit)
	}
	if options.Skip > 0 {
		query += " OFFSET ?"
		pagedArgs = append(pagedArgs, options.Skip)
	}

	rows, err := t.db.DB.QueryContext(ctx, rebindPlaceholders(query, t.db.Dialect), pagedArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		user, err := scanUserRows(rows)
		if err != nil {
			return nil, 0, err
		}
		users = append(users, *user)
	}
	return users, total, rows.Err()
}

func scanUser(row *sql.Row) (*User, error) {
	var user User
	var usernameRaw sql.NullString
	var profileImageRaw sql.NullString
	var profileBannerImageRaw sql.NullString
	var bioRaw sql.NullString
	var genderRaw sql.NullString
	var dateOfBirthRaw sql.NullString
	var timezoneRaw sql.NullString
	var presenceStateRaw sql.NullString
	var statusEmojiRaw sql.NullString
	var statusMessageRaw sql.NullString
	var statusExpiresAtRaw sql.NullInt64
	var settingsRaw sql.NullString
	var infoRaw sql.NullString
	var oauthRaw sql.NullString
	var scimRaw sql.NullString
	err := row.Scan(
		&user.ID,
		&user.Email,
		&usernameRaw,
		&user.Role,
		&user.Name,
		&profileImageRaw,
		&profileBannerImageRaw,
		&bioRaw,
		&genderRaw,
		&dateOfBirthRaw,
		&timezoneRaw,
		&presenceStateRaw,
		&statusEmojiRaw,
		&statusMessageRaw,
		&statusExpiresAtRaw,
		&settingsRaw,
		&infoRaw,
		&oauthRaw,
		&scimRaw,
		&user.LastActiveAt,
		&user.UpdatedAt,
		&user.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return hydrateUser(&user, usernameRaw, profileImageRaw, profileBannerImageRaw, bioRaw, genderRaw, dateOfBirthRaw, timezoneRaw, presenceStateRaw, statusEmojiRaw, statusMessageRaw, statusExpiresAtRaw, settingsRaw, infoRaw, oauthRaw, scimRaw)
}

func scanUserRows(rows *sql.Rows) (*User, error) {
	var user User
	var usernameRaw sql.NullString
	var profileImageRaw sql.NullString
	var profileBannerImageRaw sql.NullString
	var bioRaw sql.NullString
	var genderRaw sql.NullString
	var dateOfBirthRaw sql.NullString
	var timezoneRaw sql.NullString
	var presenceStateRaw sql.NullString
	var statusEmojiRaw sql.NullString
	var statusMessageRaw sql.NullString
	var statusExpiresAtRaw sql.NullInt64
	var settingsRaw sql.NullString
	var infoRaw sql.NullString
	var oauthRaw sql.NullString
	var scimRaw sql.NullString
	err := rows.Scan(
		&user.ID,
		&user.Email,
		&usernameRaw,
		&user.Role,
		&user.Name,
		&profileImageRaw,
		&profileBannerImageRaw,
		&bioRaw,
		&genderRaw,
		&dateOfBirthRaw,
		&timezoneRaw,
		&presenceStateRaw,
		&statusEmojiRaw,
		&statusMessageRaw,
		&statusExpiresAtRaw,
		&settingsRaw,
		&infoRaw,
		&oauthRaw,
		&scimRaw,
		&user.LastActiveAt,
		&user.UpdatedAt,
		&user.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return hydrateUser(&user, usernameRaw, profileImageRaw, profileBannerImageRaw, bioRaw, genderRaw, dateOfBirthRaw, timezoneRaw, presenceStateRaw, statusEmojiRaw, statusMessageRaw, statusExpiresAtRaw, settingsRaw, infoRaw, oauthRaw, scimRaw)
}

func hydrateUser(
	user *User,
	usernameRaw sql.NullString,
	profileImageRaw sql.NullString,
	profileBannerImageRaw sql.NullString,
	bioRaw sql.NullString,
	genderRaw sql.NullString,
	dateOfBirthRaw sql.NullString,
	timezoneRaw sql.NullString,
	presenceStateRaw sql.NullString,
	statusEmojiRaw sql.NullString,
	statusMessageRaw sql.NullString,
	statusExpiresAtRaw sql.NullInt64,
	settingsRaw sql.NullString,
	infoRaw sql.NullString,
	oauthRaw sql.NullString,
	scimRaw sql.NullString,
) (*User, error) {
	user.Username = usernameRaw.String
	user.ProfileImageURL = profileImageRaw.String
	user.ProfileBannerImageURL = profileBannerImageRaw.String
	user.Bio = bioRaw.String
	user.Gender = genderRaw.String
	if dateOfBirthRaw.Valid {
		user.DateOfBirth = dateOfBirthRaw.String
	}
	user.Timezone = timezoneRaw.String
	user.PresenceState = presenceStateRaw.String
	user.StatusEmoji = statusEmojiRaw.String
	user.StatusMessage = statusMessageRaw.String
	if statusExpiresAtRaw.Valid {
		value := statusExpiresAtRaw.Int64
		user.StatusExpiresAt = &value
	}

	var err error
	user.Settings, err = unmarshalJSONMap(settingsRaw)
	if err != nil {
		return nil, err
	}
	user.Info, err = unmarshalJSONMap(infoRaw)
	if err != nil {
		return nil, err
	}
	user.OAuth, err = unmarshalJSONMap(oauthRaw)
	if err != nil {
		return nil, err
	}
	user.SCIM, err = unmarshalJSONMap(scimRaw)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(user.ProfileImageURL) == "" {
		user.ProfileImageURL = fmt.Sprintf("/api/v1/users/%s/profile/image", user.ID)
	}
	return user, nil
}

func marshalJSONMap(value map[string]any) (any, error) {
	if value == nil {
		return nil, nil
	}
	body, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return string(body), nil
}

func unmarshalJSONMap(value sql.NullString) (map[string]any, error) {
	if !value.Valid || strings.TrimSpace(value.String) == "" {
		return nil, nil
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(value.String), &payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func cloneJSONMap(source map[string]any) map[string]any {
	if source == nil {
		return nil
	}
	body, err := json.Marshal(source)
	if err != nil {
		return nil
	}
	var target map[string]any
	if err := json.Unmarshal(body, &target); err != nil {
		return nil
	}
	return target
}

func rebindPlaceholders(query string, dialect dbinternal.Dialect) string {
	if dialect != dbinternal.DialectPostgres {
		return query
	}

	var builder strings.Builder
	builder.Grow(len(query) + 8)

	index := 1
	for _, r := range query {
		if r != '?' {
			builder.WriteRune(r)
			continue
		}
		builder.WriteString(fmt.Sprintf("$%d", index))
		index++
	}
	return builder.String()
}

func placeholdersForUsers(count int, dialect dbinternal.Dialect) string {
	out := ""
	for i := 0; i < count; i++ {
		if i > 0 {
			out += ", "
		}
		if dialect == dbinternal.DialectPostgres {
			out += "$" + strconv.Itoa(i+1)
		} else {
			out += "?"
		}
	}
	return out
}
