package database

import (
	"database/sql"

	"lab2/shared"
	"lab2/user_request"
)

func CreateTableForUserRequests(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS user_requests (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			hash TEXT NOT NULL,
			max_length INTEGER NOT NULL,
			status TEXT NOT NULL,
			result JSON
		)
	`)

	return err
}

func AddUserRequest(db *sql.DB, userRequest *user_request.UserRequest) (shared.UserRequestId, error) {
	var id shared.UserRequestId

	err := db.QueryRow(
		`INSERT INTO user_requests (hash, max_length, status)
		"VALUES ($1, $2, $3)
		RETURNING id`,
		userRequest.Hash, userRequest.MaxLength, user_request.PROCESSING).
		Scan(&id)

	return id, err
}

func GetUserRequestById(db *sql.DB, id shared.UserRequestId) (*user_request.UserRequest, error) {
	var userRequest user_request.UserRequest

	err := db.QueryRow(
		`SELECT * 
		FROM user_requests
		WHERE id = $1`,
		id).
		Scan(&userRequest.Id,
			&userRequest.Hash,
			&userRequest.MaxLength,
			&userRequest.Status,
			&userRequest.Result)

	return &userRequest, err
}

func GetUserRequestStatusById(db *sql.DB, id shared.UserRequestId) (user_request.UserStatusResponse, error) {
	var userStatusResponse user_request.UserStatusResponse

	err := db.QueryRow(
		`SELECT status, result
		FROM user_requests
		WHERE id = $1`,
		id).
		Scan(&userStatusResponse.Status, &userStatusResponse.Result)

	return userStatusResponse, err
}

func UpdateRequestStatusAndResult(db *sql.DB, id shared.UserRequestId, status user_request.UserRequestStatus, result []string) error {
	err := db.QueryRow(
		`UPDATE user_requests
		SET
			status = $1, 
			result = $2, 
		WHERE id = $3`,
		status, result, id).
		Err()

	return err
}
