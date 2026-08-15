package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/efauncodes/equipment-manager-backend/db"
	"github.com/efauncodes/equipment-manager-backend/domain"
	"github.com/efauncodes/equipment-manager-backend/repository"
	"github.com/efauncodes/equipment-manager-backend/service"
)

type apiFixture struct {
	handler http.Handler
	svc     *service.Service
	db      *sql.DB
	admin   domain.User
}

func newAPIFixture(t *testing.T) apiFixture {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "equipment.db"))
	if err != nil {
		t.Fatal(err)
	}
	store := repository.NewStore(database)
	svc := service.New(store)
	admin, err := svc.CreateUser(context.Background(), "admin@example.com", "Admin", domain.RoleAdmin, true)
	if err != nil {
		database.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return apiFixture{handler: newHandlerWithConfig(svc, true, true), svc: svc, db: database, admin: admin}
}

func TestMinimumMVPHTTPVerticalSlice(t *testing.T) {
	f := newAPIFixture(t)

	status, body := f.request(t, http.MethodPost, "/api/v1/auth/admin/magic-links", map[string]any{"email": "admin@example.com"}, "")
	if status != http.StatusAccepted {
		t.Fatalf("admin magic-link status = %d, body = %#v", status, body)
	}
	adminLoginToken := stringValue(t, body, "token")

	status, body = f.request(t, http.MethodPost, "/api/v1/auth/consume", map[string]any{"token": adminLoginToken}, "")
	if status != http.StatusOK {
		t.Fatalf("admin consume status = %d, body = %#v", status, body)
	}
	adminSession := stringValue(t, body, "session_token")
	adminData := mapValue(t, body, "user")
	if adminData["role"] != domain.RoleAdmin {
		t.Fatalf("consumed user role = %#v", adminData["role"])
	}
	assertEnvelope(t, body)

	status, body = f.request(t, http.MethodGet, "/api/v1/me", nil, adminSession)
	if status != http.StatusOK || mapValue(t, body, "data")["email"] != "admin@example.com" {
		t.Fatalf("me response = %d %#v", status, body)
	}

	status, body = f.request(t, http.MethodPost, "/api/v1/members", map[string]any{"email": "member@example.com", "display_name": "Member"}, adminSession)
	if status != http.StatusCreated {
		t.Fatalf("member create status = %d, body = %#v", status, body)
	}
	memberID := stringValue(t, body, "id")

	status, body = f.request(t, http.MethodPatch, "/api/v1/members/"+memberID, map[string]any{"display_name": "Updated Member"}, adminSession)
	if status != http.StatusOK || mapValue(t, body, "data")["display_name"] != "Updated Member" {
		t.Fatalf("member patch response = %d %#v", status, body)
	}

	status, body = f.request(t, http.MethodPost, "/api/v1/equipment", map[string]any{"serial_number": "SN-100", "type": "Harness", "size": "M", "manufacturer": "Acme", "purchase_date": "2026-08-15"}, adminSession)
	if status != http.StatusCreated {
		t.Fatalf("equipment create status = %d, body = %#v", status, body)
	}
	equipmentID := stringValue(t, body, "id")

	status, body = f.request(t, http.MethodPatch, "/api/v1/equipment/"+equipmentID, map[string]any{"manufacturer": "Acme Updated"}, adminSession)
	if status != http.StatusOK || mapValue(t, body, "data")["manufacturer"] != "Acme Updated" {
		t.Fatalf("equipment patch response = %d %#v", status, body)
	}

	status, _ = f.request(t, http.MethodGet, "/api/v1/members", nil, adminSession)
	if status != http.StatusOK {
		t.Fatalf("member list status = %d", status)
	}
	status, _ = f.request(t, http.MethodGet, "/api/v1/equipment", nil, adminSession)
	if status != http.StatusOK {
		t.Fatalf("equipment list status = %d", status)
	}

	status, body = f.request(t, http.MethodPost, "/api/v1/issuances", map[string]any{"equipment_id": equipmentID, "member_id": memberID}, adminSession)
	if status != http.StatusCreated {
		t.Fatalf("issuance create status = %d, body = %#v", status, body)
	}
	issuance := mapValue(t, body, "issuance")
	issuanceID := stringValueFromMap(t, issuance, "id")
	issuanceToken := stringValue(t, body, "confirmation_token")

	status, body = f.request(t, http.MethodPost, "/api/v1/issuances/"+issuanceID+"/confirm", map[string]any{"token": issuanceToken}, "")
	if status != http.StatusOK || mapValue(t, body, "data")["confirmation_status"] != domain.ConfirmationConfirmed {
		t.Fatalf("issuance confirm response = %d %#v", status, body)
	}

	status, body = f.request(t, http.MethodPost, "/api/v1/issuances/"+issuanceID+"/confirm", map[string]any{"token": issuanceToken}, "")
	if status != http.StatusConflict || mapValue(t, body, "error")["code"] != "TOKEN_USED" {
		t.Fatalf("reused issuance token response = %d %#v", status, body)
	}

	status, body = f.request(t, http.MethodPost, "/api/v1/issuances", map[string]any{"equipment_id": equipmentID, "member_id": memberID}, adminSession)
	if status != http.StatusConflict {
		t.Fatalf("duplicate issuance response = %d %#v", status, body)
	}

	status, body = f.request(t, http.MethodGet, "/api/v1/equipment/"+equipmentID, nil, adminSession)
	if status != http.StatusOK || mapValue(t, body, "data")["status"] != domain.StatusIssued {
		t.Fatalf("issued equipment response = %d %#v", status, body)
	}

	returnStatus, returnBody := f.request(t, http.MethodPost, "/api/v1/returns", map[string]any{"issuance_id": issuanceID}, adminSession)
	if returnStatus != http.StatusCreated {
		t.Fatalf("return create status = %d, body = %#v", returnStatus, returnBody)
	}
	returnData := mapValue(t, returnBody, "return")
	returnID := stringValueFromMap(t, returnData, "id")
	returnToken := stringValue(t, returnBody, "confirmation_token")

	status, body = f.request(t, http.MethodPost, "/api/v1/returns/"+returnID+"/confirm", map[string]any{"token": returnToken}, "")
	if status != http.StatusOK || mapValue(t, body, "data")["confirmation_status"] != domain.ConfirmationConfirmed {
		t.Fatalf("return confirm response = %d %#v", status, body)
	}

	status, body = f.request(t, http.MethodGet, "/api/v1/equipment/"+equipmentID+"/history", nil, adminSession)
	if status != http.StatusOK {
		t.Fatalf("history status = %d, body = %#v", status, body)
	}
	history := mapValueSlice(t, body, "data")
	events := map[string]bool{}
	for _, entry := range history {
		events[entry["event_type"].(string)] = true
	}
	if len(history) != 4 || !events[domain.EventCreated] || !events[domain.EventStatusChanged] || !events[domain.EventIssued] || !events[domain.EventReturned] {
		t.Fatalf("history = %#v", history)
	}

	memberLink, err := f.svc.CreateLoginLink(context.Background(), memberID)
	if err != nil {
		t.Fatal(err)
	}
	status, body = f.request(t, http.MethodPost, "/api/v1/auth/consume", map[string]any{"token": memberLink.RawToken}, "")
	if status != http.StatusOK {
		t.Fatalf("member consume status = %d, body = %#v", status, body)
	}
	memberSession := stringValue(t, body, "session_token")
	status, body = f.request(t, http.MethodGet, "/api/v1/equipment", nil, memberSession)
	if status != http.StatusForbidden || mapValue(t, body, "error")["code"] != "FORBIDDEN" {
		t.Fatalf("member equipment access = %d %#v", status, body)
	}

	productionHandler := newHandlerWithConfig(f.svc, false, false)
	status, body = requestHandler(t, productionHandler, http.MethodPost, "/api/v1/auth/admin/magic-links", map[string]any{"email": "admin@example.com"}, "")
	if status != http.StatusAccepted {
		t.Fatalf("production magic-link status = %d, body = %#v", status, body)
	}
	if _, exists := mapValue(t, body, "data")["token"]; exists {
		t.Fatal("production magic-link response exposed a raw token")
	}

	status, body = f.request(t, http.MethodPost, "/api/v1/auth/logout", nil, adminSession)
	if status != http.StatusOK {
		t.Fatalf("logout status = %d, body = %#v", status, body)
	}
	status, body = f.request(t, http.MethodGet, "/api/v1/me", nil, adminSession)
	if status != http.StatusUnauthorized || mapValue(t, body, "error")["code"] != "UNAUTHORIZED" {
		t.Fatalf("revoked session response = %d %#v", status, body)
	}
}

func TestAPIRejectsMissingAuthenticationWithStructuredError(t *testing.T) {
	f := newAPIFixture(t)
	status, body := f.request(t, http.MethodGet, "/api/v1/members", nil, "")
	if status != http.StatusUnauthorized {
		t.Fatalf("status = %d, body = %#v", status, body)
	}
	assertEnvelope(t, body)
	if mapValue(t, body, "error")["code"] != "UNAUTHORIZED" {
		t.Fatalf("error = %#v", body["error"])
	}
}

func (f apiFixture) request(t *testing.T, method, path string, body any, bearer string) (int, map[string]any) {
	t.Helper()
	return requestHandler(t, f.handler, method, path, body, bearer)
}

func requestHandler(t *testing.T, handler http.Handler, method, path string, body any, bearer string) (int, map[string]any) {
	t.Helper()
	var reader io.Reader = http.NoBody
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(encoded)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	var decoded map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("decode %s %s response: %v; body=%s", method, path, err, res.Body.String())
	}
	return res.Code, decoded
}

func assertEnvelope(t *testing.T, body map[string]any) {
	t.Helper()
	meta := mapValue(t, body, "meta")
	if meta["request_id"] == "" || meta["local_time"] == "" {
		t.Fatalf("invalid response meta: %#v", meta)
	}
}

func mapValue(t *testing.T, body map[string]any, key string) map[string]any {
	t.Helper()
	value, ok := nestedValue(body, key).(map[string]any)
	if !ok {
		t.Fatalf("%s is not an object: %#v", key, nestedValue(body, key))
	}
	return value
}

func mapValueSlice(t *testing.T, body map[string]any, key string) []map[string]any {
	t.Helper()
	value, ok := body[key].([]any)
	if !ok {
		t.Fatalf("%s is not an array: %#v", key, body[key])
	}
	result := make([]map[string]any, 0, len(value))
	for _, item := range value {
		object, ok := item.(map[string]any)
		if !ok {
			t.Fatalf("array item is not an object: %#v", item)
		}
		result = append(result, object)
	}
	return result
}

func stringValue(t *testing.T, body map[string]any, key string) string {
	t.Helper()
	return stringValueFromMap(t, body, key)
}

func stringValueFromMap(t *testing.T, body map[string]any, key string) string {
	t.Helper()
	value, ok := nestedValue(body, key).(string)
	if !ok || value == "" {
		t.Fatalf("%s is not a non-empty string: %#v", key, nestedValue(body, key))
	}
	return value
}

func nestedValue(body map[string]any, key string) any {
	if value, ok := body[key]; ok {
		return value
	}
	if data, ok := body["data"].(map[string]any); ok {
		return data[key]
	}
	return nil
}
