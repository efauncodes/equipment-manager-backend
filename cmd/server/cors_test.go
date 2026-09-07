package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

const stagingFrontendOrigin = "https://equipment.sentient-octopus.dev"

func TestCORSPreflightUsesExactAllowlist(t *testing.T) {
	f := newAPIFixture(t)
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/me", nil)
	req.Header.Set("Origin", stagingFrontendOrigin)
	req.Header.Set("Access-Control-Request-Method", http.MethodGet)
	req.Header.Set("Access-Control-Request-Headers", "Authorization, Content-Type, X-Request-ID")
	res := httptest.NewRecorder()

	f.handler.ServeHTTP(res, req)

	if res.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusNoContent)
	}
	if res.Body.Len() != 0 {
		t.Fatalf("body = %q, want empty body", res.Body.String())
	}
	if got := res.Header().Get("Access-Control-Allow-Origin"); got != stagingFrontendOrigin {
		t.Fatalf("allow origin = %q, want %q", got, stagingFrontendOrigin)
	}
	if got := res.Header().Get("Access-Control-Allow-Methods"); got != "GET, POST, PATCH, OPTIONS" {
		t.Fatalf("allow methods = %q", got)
	}
	if got := res.Header().Get("Access-Control-Allow-Headers"); got != "Authorization, Content-Type, X-Request-ID" {
		t.Fatalf("allow headers = %q", got)
	}
	if got := res.Header().Get("Access-Control-Max-Age"); got != "600" {
		t.Fatalf("max age = %q, want 600", got)
	}
	if got := res.Header().Get("Vary"); got != "Origin" {
		t.Fatalf("vary = %q, want Origin", got)
	}
	if got := res.Header().Get("Access-Control-Allow-Credentials"); got != "" {
		t.Fatalf("allow credentials = %q, want header omitted", got)
	}
}

func TestCORSHeadersArePresentOnAllowedAPIResponses(t *testing.T) {
	f := newAPIFixture(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	req.Header.Set("Origin", stagingFrontendOrigin)
	res := httptest.NewRecorder()

	f.handler.ServeHTTP(res, req)

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusUnauthorized)
	}
	if got := res.Header().Get("Access-Control-Allow-Origin"); got != stagingFrontendOrigin {
		t.Fatalf("allow origin = %q, want %q", got, stagingFrontendOrigin)
	}
	if got := res.Header().Get("Vary"); got != "Origin" {
		t.Fatalf("vary = %q, want Origin", got)
	}
	if got := res.Header().Get("Access-Control-Allow-Credentials"); got != "" {
		t.Fatalf("allow credentials = %q, want header omitted", got)
	}
}

func TestCORSRejectsUnexpectedOriginsAndCookies(t *testing.T) {
	tests := []struct {
		name   string
		origin string
		cookie string
	}{
		{name: "unexpected origin", origin: "https://unexpected.example"},
		{name: "cookie credentials", origin: stagingFrontendOrigin, cookie: "session=unexpected"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newAPIFixture(t)
			req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
			req.Header.Set("Origin", tt.origin)
			if tt.cookie != "" {
				req.Header.Set("Cookie", tt.cookie)
			}
			res := httptest.NewRecorder()

			f.handler.ServeHTTP(res, req)

			if res.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want %d", res.Code, http.StatusForbidden)
			}
			if got := res.Header().Get("Access-Control-Allow-Origin"); got != "" {
				t.Fatalf("allow origin = %q, want header omitted", got)
			}
		})
	}
}

func TestCORSRejectsInvalidPreflight(t *testing.T) {
	f := newAPIFixture(t)
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/me", nil)
	req.Header.Set("Origin", stagingFrontendOrigin)
	req.Header.Set("Access-Control-Request-Method", http.MethodDelete)
	req.Header.Set("Access-Control-Request-Headers", "Authorization, X-Not-Allowed")
	res := httptest.NewRecorder()

	f.handler.ServeHTTP(res, req)

	if res.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusForbidden)
	}
	if got := res.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("allow origin = %q, want header omitted", got)
	}
}

func TestCORSDoesNotApplyToHealthEndpoint(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("Origin", stagingFrontendOrigin)
	res := httptest.NewRecorder()

	newHandler().ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}
	if got := res.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("allow origin = %q, want header omitted", got)
	}
}
