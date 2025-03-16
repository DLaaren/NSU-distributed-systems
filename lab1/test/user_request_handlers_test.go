package test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"lab1/coordinator"
	"lab1/shared"
)

/*
 * Missing requestId -> returns 400 Bad Request
 * Invalid requestId -> returns 500 Internal Server Error
 * Valid requestId -> returns 200 OK with the expected JSON
 */
func TestGetRequestStatusHandler(t *testing.T) {
	mockCoordinator := &MockCoordinator{}

	req, err := http.NewRequest("GET", "/api/hash/status?requestId=123", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()

	handler := coordinator.GetRequestStatusHandler(mockCoordinator)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	expected := coordinator.UserStatusResponse{
		Status: coordinator.READY,
		Result: "aboba",
	}

	var response coordinator.UserStatusResponse

	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatal("failed to decode response:", err)
	}
	if response != expected {
		t.Errorf("handler returned unexpected body: got %v want %v", response, expected)
	}

	/* Test missing requestId */
	req, err = http.NewRequest("GET", "/api/hash/status", nil)
	if err != nil {
		t.Fatal(err)
	}
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code for missing requestId: got %v want %v", status, http.StatusBadRequest)
	}

	/* Test invalid requestId */
	req, err = http.NewRequest("GET", "/api/hash/status?requestId=invalid", nil)
	if err != nil {
		t.Fatal(err)
	}
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if status := rr.Code; status != http.StatusInternalServerError {
		t.Errorf("handler returned wrong status code for invalid requestId: got %v want %v", status, http.StatusInternalServerError)
	}
}

func TestSubmitRequestCrackHandler(t *testing.T) {
	mockCoordinator := &MockCoordinator{}

	userRequest := coordinator.UserRequest{
		Hash:      "5f4dcc3b5aa765d61d8327deb882cf99",
		MaxLength: 5,
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(userRequest); err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequest("POST", "/api/hash/crack", &buf)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()

	handler := coordinator.SubmitRequestCrackHandler(mockCoordinator)
	handler.ServeHTTP(rr, req)

	// Check the status code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	// Check the response body
	expected := coordinator.UserResponse{
		RequestId: shared.UserRequestId(123),
	}
	var response coordinator.UserResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatal("failed to decode response:", err)
	}
	if response != expected {
		t.Errorf("handler returned unexpected body: got %v want %v", response, expected)
	}

	// Test invalid JSON body
	req, err = http.NewRequest("POST", "/crack", strings.NewReader("invalid json"))
	if err != nil {
		t.Fatal(err)
	}
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code for invalid JSON: got %v want %v", status, http.StatusBadRequest)
	}
}
