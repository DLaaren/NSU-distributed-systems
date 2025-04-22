package database

import (
	"database/sql"

	"lab2/request"
	"lab2/shared"

	"github.com/lib/pq"
)

func CreateTableForUserRequests(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS user_requests (
			id SERIAL PRIMARY KEY,
			hash TEXT NOT NULL,
			max_length INTEGER NOT NULL,
			status TEXT NOT NULL,
			result TEXT[] DEFAULT ARRAY['']::TEXT[]
		)
	`)

	return err
}

func AddUserRequest(db *sql.DB, userRequest *prequest.UserRequest) (shared.UserRequestId, error) {
	var id shared.UserRequestId

	err := db.QueryRow(
		`INSERT INTO user_requests (hash, max_length, status)
		VALUES ($1, $2, $3)
		RETURNING id`,
		userRequest.Hash, userRequest.MaxLength, prequest.PROCESSING).
		Scan(&id)

	return id, err
}

func GetUserRequestById(db *sql.DB, id shared.UserRequestId) (*prequest.UserRequest, error) {
	var userRequest prequest.UserRequest

	err := db.QueryRow(
		`SELECT * 
		FROM user_requests
		WHERE id = $1`,
		id).
		Scan(&userRequest.Id,
			&userRequest.Hash,
			&userRequest.MaxLength,
			&userRequest.Status,
			pq.Array(&userRequest.Result))

	return &userRequest, err
}

func GetUserRequestStatusById(db *sql.DB, id shared.UserRequestId) (prequest.UserStatusResponse, error) {
	var userStatusResponse prequest.UserStatusResponse

	err := db.QueryRow(
		`SELECT status, result
		FROM user_requests
		WHERE id = $1`,
		id).
		Scan(&userStatusResponse.Status, pq.Array(&userStatusResponse.Result))

	return userStatusResponse, err
}

func UpdateRequestStatusAndResult(db *sql.DB, id shared.UserRequestId, status prequest.UserRequestStatus, result []string) error {
	err := db.QueryRow(
		`UPDATE user_requests
		SET
			status = $1, 
			result = $2 
		WHERE id = $3`,
		status, pq.Array(result), id).
		Err()

	return err
}

func GetUserRequestProcessing(db *sql.DB) ([]*prequest.UserRequest, error) {
	rows, err := db.Query(`
		SELECT *
		FROM user_requests
		WHERE status = 'PROCESSING'
	`)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var requests []*prequest.UserRequest
	for rows.Next() {
		var request prequest.UserRequest
		err = rows.Scan(
			&request.Id,
			&request.Hash,
			&request.MaxLength,
			&request.Status,
			&request.Result,
		)
		if err != nil {
			return nil, err
		}

		requests = append(requests, &request)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return requests, err
}
