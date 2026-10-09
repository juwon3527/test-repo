package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMatchCentreEndpoint(t *testing.T) {
	response := httptest.NewRecorder()
	newServer().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/match-centre", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if got := response.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}
	var centre MatchCentre
	if err := json.Unmarshal(response.Body.Bytes(), &centre); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !centre.Demo || centre.Match.Home != "Chelsea" || len(centre.Fixtures) == 0 {
		t.Fatalf("unexpected match centre payload: %+v", centre)
	}
}

func TestHomePageIsServed(t *testing.T) {
	response := httptest.NewRecorder()
	newServer().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if !strings.Contains(response.Body.String(), "The Bridge") {
		t.Fatal("home page does not contain the site identity")
	}
}