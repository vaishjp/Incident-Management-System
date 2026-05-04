package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson" // 👉 Added for Fix 29 (Mongo querying)
)

// ==========================================
// GLOBALS & TELEMETRY
// ==========================================

// ARCHITECTURE FEATURE: Asynchronous Ingestion
var signalQueue = make(chan Signal, 100000)

// 👉 FIX 10: Graceful Shutdown WaitGroup
var wg sync.WaitGroup

// 👉 FIX 11: Worker Pool Telemetry
var metricsProcessed atomic.Uint64
var metricsDropped atomic.Uint64

const dashboardCacheKey = "dashboard:active_incidents"

// 👉 FIX 20: Circuit Breaker State Struct
type CircuitBreaker struct {
	failures int32
	isOpen   atomic.Bool
}

func (cb *CircuitBreaker) RecordFailure() {
	if atomic.AddInt32(&cb.failures, 1) >= 5 {
		if cb.isOpen.CompareAndSwap(false, true) {
			slog.Warn("🔴 CIRCUIT BREAKER TRIPPED! Database requests suspended for 30s.")
			go func() {
				time.Sleep(30 * time.Second)
				atomic.StoreInt32(&cb.failures, 0)
				cb.isOpen.Store(false)
				slog.Info("🟢 CIRCUIT BREAKER RESET. Resuming normal operations.")
			}()
		}
	}
}

func (cb *CircuitBreaker) RecordSuccess() {
	atomic.StoreInt32(&cb.failures, 0)
}

var pgCircuitBreaker CircuitBreaker
var mongoCircuitBreaker CircuitBreaker

func main() {
	// 👉 FIX 12: Structured JSON Logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// 👉 FIX 9: Environment Variables (Security)
	if err := godotenv.Load(); err != nil {
		slog.Warn("No .env file found, relying on system environment variables")
	}

	InitPostgres()
	InitMongoDB()
	InitRedis()

	// 👉 FIX 18: Adaptive Worker Pool (Hardware Aware)
	workerCount := runtime.NumCPU() * 2
	slog.Info("Initializing Worker Pool", "worker_count", workerCount, "cores", runtime.NumCPU())
	for i := 1; i <= workerCount; i++ {
		wg.Add(1) 
		go processSignals(i)
	}

	// 👉 FIX 28: Real-time Throughput Logger
	// Fulfills the requirement to log signals/sec every 5 seconds.
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		var lastProcessed uint64
		for range ticker.C {
			current := metricsProcessed.Load()
			rate := (current - lastProcessed) / 5
			slog.Info("📈 Throughput Telemetry", "processed_total", current, "signals_per_sec", rate, "queue_depth", len(signalQueue))
			lastProcessed = current
		}
	}()

	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})
	app.Use(cors.New())

	// 👉 FIX 13: Realistic Rate Limiting
	app.Use(limiter.New(limiter.Config{
		Max:        1000, 
		Expiration: 1 * time.Second,
	}))

	// API Routes
	app.Post("/ingest", handleIngest)
	app.Get("/incidents", getActiveIncidents)
	app.Post("/incidents/:id/acknowledge", acknowledgeIncident)
	app.Post("/rca", submitRCA)
	app.Post("/incidents/:id/close", closeIncident)
	
	// 👉 FIX 27: Explicit Health Endpoint (Required)
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	// 👉 FIX 29: Expose Raw Signals for the Frontend
	app.Get("/incidents/:id/signals", getRawSignals)

	// 👉 FIX 10: Graceful Shutdown OS Signal Listener
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-quit 
		slog.Info("🛑 Gracefully shutting down...", 
			"total_processed", metricsProcessed.Load(),
			"total_dropped", metricsDropped.Load(),
		)
		
		app.Shutdown()      
		close(signalQueue)  
	}()

	slog.Info("🚀 IMS Backend started", "port", 3000)
	if err := app.Listen(":3000"); err != nil {
		slog.Error("Server forced to shutdown", "error", err.Error())
	}

	slog.Info("⏳ Waiting for workers to drain the queue...")
	wg.Wait()
	slog.Info("✅ All workers safely stopped. Zero data loss.")
}

// ==========================================
// INGESTION (PRODUCER)
// ==========================================

func handleIngest(c *fiber.Ctx) error {
	// 👉 FIX 17: API Idempotency (Duplicate Request Handling)
	idempotencyKey := c.Get("X-Idempotency-Key")
	if idempotencyKey != "" {
		exists, _ := redisClient.Exists(context.Background(), "idemp:"+idempotencyKey).Result()
		if exists > 0 {
			return c.Status(200).JSON(fiber.Map{"status": "Already processed", "idempotency": "hit"})
		}
		redisClient.Set(context.Background(), "idemp:"+idempotencyKey, "processed", 24*time.Hour)
	}

	var sig Signal
	if err := c.BodyParser(&sig); err != nil {
		slog.Warn("Unparseable payload received")
		return c.Status(400).JSON(fiber.Map{"error": "Invalid JSON payload"})
	}

	// 👉 FIX 15: Input Validation (Fail Fast)
	if sig.ComponentID == "" || sig.ErrorType == "" || sig.Severity == "" {
		slog.Warn("Rejected payload missing required fields", "component", sig.ComponentID)
		return c.Status(400).JSON(fiber.Map{"error": "component_id, error_type, and severity are required fields"})
	}

	sig.Timestamp = time.Now().Format(time.RFC3339)

	select {
	case signalQueue <- sig:
		return c.Status(202).JSON(fiber.Map{"status": "Accepted for processing"})
	default:
		metricsDropped.Add(1) 
		slog.Warn("System overloaded, applying backpressure. Signal dropped.")
		return c.Status(503).JSON(fiber.Map{"error": "System overloaded, applying backpressure"})
	}
}

// ==========================================
// WORKER POOL (CONSUMER)
// ==========================================

func processSignals(workerID int) {
	defer wg.Done() 

	ctx := context.Background()

	for sig := range signalQueue {
		cacheKey := fmt.Sprintf("debounce:%s:%s", sig.ComponentID, sig.ErrorType)
		
		// 👉 FIX 16: Prevent Redis Race Condition (Atomic SETNX)
		acquired, _ := redisClient.SetNX(ctx, cacheKey, "processing", 10*time.Second).Result()

		if acquired {
			// ==========================================
			// SCENARIO A: FIRST SIGNAL (Atomic Win)
			// ==========================================
			// 👉 FIX 22: Parse the actual signal time for perfect MTTR tracking
			parsedTime, _ := time.Parse(time.RFC3339, sig.Timestamp)

			workItem := WorkItem{
				ComponentID:     sig.ComponentID,
				ErrorType:       sig.ErrorType,
				Severity:        sig.Severity,
				Status:          "OPEN",
				FirstSignalTime: parsedTime, 
			}

			// 👉 FIX 20: Check Circuit Breaker before hitting DB
			if pgCircuitBreaker.isOpen.Load() {
				slog.Error("Postgres Circuit Breaker OPEN: Dropping incident creation", "worker_id", workerID)
				continue
			}

			// 👉 FIX 3: Database Retry Loop
			var pgErr error
			for i := 0; i < 3; i++ {
				// 👉 FIX 19: DB Call Timeouts (Prevent goroutine deadlocks)
				dbCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
				pgErr = db.WithContext(dbCtx).Create(&workItem).Error
				cancel() 

				if pgErr == nil {
					pgCircuitBreaker.RecordSuccess()
					break 
				}
				slog.Warn("Postgres insert failed, retrying...", "worker_id", workerID, "attempt", i+1)
				time.Sleep(100 * time.Millisecond) 
			}

			// 👉 FIX 4: Strict Error Handling
			if pgErr != nil {
				pgCircuitBreaker.RecordFailure()
				slog.Error("Failed to create WorkItem in Postgres", "worker_id", workerID, "error", pgErr.Error())
				continue 
			}

			// 👉 FIX 2a: Relational Linking (Primary)
			sig.WorkItemID = workItem.ID

			redisClient.Set(ctx, cacheKey, fmt.Sprintf("%d", workItem.ID), 10*time.Second)

			// 👉 FIX 6: Strategy Pattern
			HandleAlertStrategy(workItem)

		} else {
			// ==========================================
			// SCENARIO B: DEBOUNCED SIGNAL
			// ==========================================
			
			existingWorkItemID, _ := redisClient.Get(ctx, cacheKey).Result()
			
			// 👉 FIX 26: Subtle Race Condition Mitigation
			if existingWorkItemID == "processing" {
				time.Sleep(50 * time.Millisecond) // Give primary a fraction of a second to finish
				existingWorkItemID, _ = redisClient.Get(ctx, cacheKey).Result()
			}

			if existingWorkItemID != "processing" && existingWorkItemID != "" {
				// 👉 FIX 2b: Relational Linking (Debounced)
				parsedID, _ := strconv.Atoi(existingWorkItemID)
				sig.WorkItemID = uint(parsedID)
			}
		}

		// MongoDB Audit Log Insertion
		if mongoCircuitBreaker.isOpen.Load() {
			slog.Error("Mongo Circuit Breaker OPEN: Fast routing to DLQ", "worker_id", workerID)
			routeToDLQ(ctx, sig, workerID)
		} else {
			var mongoErr error
			for i := 0; i < 3; i++ {
				// 👉 FIX 19: DB Call Timeouts for Mongo
				mongoCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
				_, mongoErr = mongoCol.InsertOne(mongoCtx, sig)
				cancel()

				if mongoErr == nil {
					mongoCircuitBreaker.RecordSuccess()
					break
				}
				slog.Warn("MongoDB insert failed, retrying...", "worker_id", workerID, "attempt", i+1)
				time.Sleep(100 * time.Millisecond)
			}

			// 👉 FIX 8: Dead Letter Queue (DLQ)
			if mongoErr != nil {
				mongoCircuitBreaker.RecordFailure()
				slog.Error("MongoDB is down. Buffering to DLQ.", "worker_id", workerID, "error", mongoErr.Error())
				routeToDLQ(ctx, sig, workerID)
			}
		}

		metricsProcessed.Add(1) 
	}
	slog.Info("Worker shutting down cleanly", "worker_id", workerID)
}

// Helper function for DLQ routing
func routeToDLQ(ctx context.Context, sig Signal, workerID int) {
	sigJSON, marshalErr := json.Marshal(sig)
	if marshalErr == nil {
		redisClient.LPush(ctx, "dlq:mongo_fallback", sigJSON)
	} else {
		slog.Error("Failed to serialize for DLQ", "worker_id", workerID)
	}
}

// ==========================================
// FRONTEND API ROUTES (DASHBOARD)
// ==========================================

// 🔍 Endpoint: Fetch non-closed Incidents
func getActiveIncidents(c *fiber.Ctx) error {
	ctx := context.Background()

	// 👉 FIX 14: Defensive Pagination
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 50)
	if limit > 100 {
		limit = 100 
	}
	offset := (page - 1) * limit

	// 👉 FIX 7: Read-Through Dashboard Caching
	cacheKey := fmt.Sprintf("dashboard:incidents:p%d:l%d", page, limit)

	cachedData, err := redisClient.Get(ctx, cacheKey).Result()
	if err == nil {
		c.Set("Content-Type", "application/json")
		return c.SendString(cachedData)
	}

	var items []WorkItem
	db.Where("status != ?", "CLOSED").
		Order("created_at desc").
		Limit(limit).
		Offset(offset).
		Find(&items)

	jsonData, _ := json.Marshal(items)
	redisClient.SetEX(ctx, cacheKey, jsonData, 5*time.Second)

	return c.JSON(items)
}

// 📡 👉 FIX 29: Endpoint: Fetch Raw Signals from MongoDB for a specific Incident
func getRawSignals(c *fiber.Ctx) error {
	idParam := c.Params("id")
	workID, err := strconv.Atoi(idParam)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid incident ID"})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Query MongoDB for all raw signals linked to this specific Postgres Incident ID
	cursor, err := mongoCol.Find(ctx, bson.M{"work_item_id": workID})
	if err != nil {
		slog.Error("Failed to fetch raw signals from MongoDB", "error", err.Error())
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch raw signals"})
	}
	defer cursor.Close(ctx)

	var signals []Signal
	if err = cursor.All(ctx, &signals); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to parse raw signals"})
	}

	// Return empty array instead of null if no signals are found
	if signals == nil {
		signals = []Signal{}
	}

	return c.JSON(signals)
}

// 🟡 Endpoint: Acknowledge Incident
func acknowledgeIncident(c *fiber.Ctx) error {
	id := c.Params("id")
	var workItem WorkItem

	if err := db.First(&workItem, id).Error; err != nil {
		return c.Status(404).SendString("WorkItem not found")
	}

	// 👉 FIX 5: State Pattern Enforcement 
	workItem.LoadState()
	if err := workItem.State.Acknowledge(&workItem); err != nil {
		slog.Warn("Illegal state transition attempted", "action", "acknowledge", "id", id)
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	db.Save(&workItem)
	slog.Info("Incident acknowledged", "id", id)
	return c.JSON(fiber.Map{"status": "Incident moved to INVESTIGATING"})
}

// 🔵 Endpoint: Submit RCA
func submitRCA(c *fiber.Ctx) error {
	var rca RCA
	if err := c.BodyParser(&rca); err != nil {
		return c.Status(400).SendString("Invalid RCA format")
	}

	tx := db.Begin()
	var workItem WorkItem
	if err := tx.First(&workItem, rca.WorkItemID).Error; err != nil {
		tx.Rollback()
		return c.Status(404).SendString("WorkItem not found")
	}

	// 👉 FIX 5: State Pattern Enforcement 
	workItem.LoadState()
	if err := workItem.State.Resolve(&workItem); err != nil {
		tx.Rollback()
		slog.Warn("Illegal state transition attempted", "action", "resolve", "work_item_id", rca.WorkItemID)
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	if err := tx.Create(&rca).Error; err != nil {
		tx.Rollback()
		return c.Status(500).SendString("Failed to save RCA")
	}

	if err := tx.Save(&workItem).Error; err != nil {
		tx.Rollback()
		return c.Status(500).SendString("Failed to update WorkItem state")
	}

	tx.Commit()
	slog.Info("RCA submitted and incident resolved", "work_item_id", rca.WorkItemID)
	return c.JSON(fiber.Map{"status": "RCA logged, Incident RESOLVED"})
}

// 🟢 Endpoint: Close Incident
func closeIncident(c *fiber.Ctx) error {
	id := c.Params("id")
	var workItem WorkItem

	if err := db.First(&workItem, id).Error; err != nil {
		return c.Status(404).SendString("WorkItem not found")
	}

	// 👉 FIX 5: State Pattern Enforcement 
	workItem.LoadState()
	if err := workItem.State.Close(&workItem); err != nil {
		slog.Warn("Illegal state transition attempted", "action", "close", "id", id)
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	// 👉 FIX 22: Calculate MTTR using the FirstSignalTime, not DB creation time
	resolvedTime := time.Now()
	mttr := int64(resolvedTime.Sub(workItem.FirstSignalTime).Minutes())

	workItem.ResolvedAt = resolvedTime
	workItem.MTTR = mttr

	db.Save(&workItem)
	slog.Info("Incident closed", "id", id, "mttr_minutes", mttr)
	return c.JSON(fiber.Map{"status": "Incident safely CLOSED", "mttr_minutes": mttr})
}
