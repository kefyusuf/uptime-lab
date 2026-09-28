package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	"uuid"
)

func TestProductionMonitorIDGeneratorReturnsVersion7Identity(t *testing.T) {
	id, err := newMonitorID()
	if err != nil {
		t.Fatalf("newMonitorID() error = %v", err)
	}
	if id.UUID() == uuid.Nil() {
		t.Fatal("newMonitorID() returned nil UUID")
	}
	if got := id.UUID()[6] >> 4; got != 7 {
		t.Fatalf("newMonitorID() UUID version = %d, want 7", got)
	}
}

func TestProductionCheckIDGeneratorReturnsVersion7Identity(t *testing.T) {
	id, err := newCheckID()
	if err != nil {
		t.Fatalf("newCheckID() error = %v", err)
	}
	if id.UUID() == uuid.Nil() {
		t.Fatal("newCheckID() returned nil UUID")
	}
	if got := id.UUID()[6] >> 4; got != 7 {
		t.Fatalf("newCheckID() UUID version = %d, want 7", got)
	}
}

func TestProductRouterDelegatesInternalCheckerPathsWithoutChangingPublicFallback(t *testing.T) {
	publicCalls := 0
	internalCalls := 0

	publicHandler := http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		publicCalls++
		writer.WriteHeader(http.StatusCreated)
	})
	internalHandler := http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		internalCalls++
		writer.WriteHeader(http.StatusAccepted)
	})
	router := newProductRouter(publicHandler, internalHandler)

	for _, test := range []struct {
		name         string
		path         string
		wantStatus   int
		wantPublic   int
		wantInternal int
	}{
		{name: "public monitor collection", path: "/monitors", wantStatus: http.StatusCreated, wantPublic: 1},
		{name: "internal claim", path: "/internal/checks/claim", wantStatus: http.StatusAccepted, wantInternal: 1},
		{name: "internal result", path: "/internal/checks/018f22d3-1d6a-7cc0-a37b-46fc3fafd101/result", wantStatus: http.StatusAccepted, wantInternal: 1},
		{name: "non-internal similar prefix", path: "/internal/checks", wantStatus: http.StatusCreated, wantPublic: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			publicCalls = 0
			internalCalls = 0
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, test.path, nil)

			router.ServeHTTP(recorder, request)

			if recorder.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, test.wantStatus)
			}
			if publicCalls != test.wantPublic || internalCalls != test.wantInternal {
				t.Fatalf(
					"handler calls public/internal = %d/%d, want %d/%d",
					publicCalls,
					internalCalls,
					test.wantPublic,
					test.wantInternal,
				)
			}
		})
	}
}

func TestProductionClockReturnsUTCMicrosecondPrecision(t *testing.T) {
	got := productionClock()
	if got.IsZero() {
		t.Fatal("productionClock() returned zero time")
	}
	if got.Location() != time.UTC {
		t.Fatalf("productionClock() location = %v, want UTC", got.Location())
	}
	if got.Nanosecond()%int(time.Microsecond) != 0 {
		t.Fatalf("productionClock() nanoseconds = %d, want microsecond precision", got.Nanosecond())
	}
}
