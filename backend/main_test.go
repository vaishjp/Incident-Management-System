package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// 👉 FIX 24: Unit Tests for RCA Validation & State Machine
func setupTestApp() *fiber.App {
	// Use in-memory SQLite for blazing fast isolated tests
	db, _ = gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	db.AutoMigrate(&WorkItem{}, &RCA{})

	app := fiber.New()
	app.Post("/rca", submitRCA) // Expose the handler we want to test
	return app
}

func TestSubmitRCA_ValidState(t *testing.T) {
	app := setupTestApp()

	// Seed an INVESTIGATING incident (Valid state for RCA)
	db.Create(&WorkItem{Model: gorm.Model{ID: 1}, Status: "INVESTIGATING"})

	rcaPayload := map[string]interface{}{
		"work_item_id": 1,
		"root_cause":   "Memory Leak",
		"fix_applied":  "Restarted Pods",
	}
	body, _ := json.Marshal(rcaPayload)

	req := httptest.NewRequest("POST", "/rca", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 OK, got %d", resp.StatusCode)
	}

	// Verify DB state changed to RESOLVED
	var item WorkItem
	db.First(&item, 1)
	if item.Status != "RESOLVED" {
		t.Errorf("Expected state to be RESOLVED, got %s", item.Status)
	}
}

func TestSubmitRCA_InvalidState(t *testing.T) {
	app := setupTestApp()

	// Seed an OPEN incident (Invalid state for RCA - must be Acknowledged first)
	db.Create(&WorkItem{Model: gorm.Model{ID: 2}, Status: "OPEN"})

	rcaPayload := map[string]interface{}{
		"work_item_id": 2,
		"root_cause":   "Network blip",
	}
	body, _ := json.Marshal(rcaPayload)

	req := httptest.NewRequest("POST", "/rca", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)

	// 👉 Enforcing the State Pattern: Should block with 400 Bad Request
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request for illegal state transition, got %d", resp.StatusCode)
	}
}
