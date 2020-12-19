package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
)

func TestGetHandler(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(postHandler)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusMethodNotAllowed {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusMethodNotAllowed)
	}

}

func TestPostHandlerOk(t *testing.T) {

	var jsonStr = []byte(`["https://ya.ru", "https://google.ru"]`)

	req, err := http.NewRequest(http.MethodPost, "/", bytes.NewBuffer(jsonStr))
	req.Header.Set("Content-Type", "application/json")
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(postHandler)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	match, _ := regexp.MatchString(`{"https:\/\/google\.ru":".*","https:\/\/ya\.ru":".*"}`, rr.Body.String())
	if !match {
		t.Errorf("handler returned unexpected body: got %v want %v",
			rr.Body.String(), `{"https:\/\/google\.ru":".*","https:\/\/ya\.ru":".*"}`)
	}

}

func TestPostHandlerBadRequest(t *testing.T) {

	var jsonStr = []byte(`["https://ya.ru", "https://google.ru"], "https://ya.ru", "https://google.ru", "https://ya.ru", "https://google.ru", "https://ya.ru", "https://google.ru", "https://ya.ru", "https://google.ru", "https://ya.ru", "https://google.ru", "https://ya.ru", "https://google.ru", "https://ya.ru", "https://google.ru", "https://ya.ru", "https://google.ru", "https://ya.ru", "https://google.ru", "https://ya.ru"`)

	req, err := http.NewRequest(http.MethodPost, "/", bytes.NewBuffer(jsonStr))
	req.Header.Set("Content-Type", "application/json")
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(postHandler)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusBadRequest)
	}

}
