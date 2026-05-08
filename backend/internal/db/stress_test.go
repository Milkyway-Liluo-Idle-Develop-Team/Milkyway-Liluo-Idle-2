package db_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/auth"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/config"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/db"
	dbgen "github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/db/gen"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/gameconfig"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/playerinit"
)

func init() {
	if err := gameconfig.Load(); err != nil {
		panic("gameconfig: " + err.Error())
	}
}

type stressResult struct {
	concurrency  int
	duration     time.Duration
	success      int64
	failures     int64
	busyErrors   int64
	uniqueErrors int64
	otherErrors  int64
	minLatency   time.Duration
	maxLatency   time.Duration
	avgLatency   time.Duration
	p95Latency   time.Duration
	p99Latency   time.Duration
}

func runStressWorkload(t *testing.T, concurrency int, bcryptCost int, maxOpenConns int) *stressResult {
	ctx := context.Background()
	dbFile := fmt.Sprintf("_stress_%d.db", concurrency)
	_ = os.Remove(dbFile)
	_ = os.Remove(dbFile + "-shm")
	_ = os.Remove(dbFile + "-wal")
	defer os.Remove(dbFile)
	defer os.Remove(dbFile + "-shm")
	defer os.Remove(dbFile + "-wal")

	database, err := db.Open(ctx, config.DB{
		URL:          fmt.Sprintf("file:%s?_pragma=foreign_keys(1)", dbFile),
		MaxOpenConns: int32(maxOpenConns),
		MaxIdleConns: int32(maxOpenConns),
		AutoMigrate:  true,
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer database.Close()

	svc := auth.NewService(database, config.Auth{BcryptCost: bcryptCost})

	var wg sync.WaitGroup
	var successCount, failCount, busyCount, uniqueCount int64
	latencies := make([]int64, 0, concurrency)
	var latMu sync.Mutex

	start := time.Now()

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			latStart := time.Now()

			u, err := svc.Register(ctx, fmt.Sprintf("user%d_%d", idx, time.Now().UnixNano()), fmt.Sprintf("u%d@test.com", idx), "password123")
			if err != nil {
				atomic.AddInt64(&failCount, 1)
				msg := strings.ToLower(err.Error())
				if strings.Contains(msg, "database is locked") || strings.Contains(msg, "busy") {
					atomic.AddInt64(&busyCount, 1)
				} else if strings.Contains(msg, "unique") {
					atomic.AddInt64(&uniqueCount, 1)
				}
				return
			}

			if err := playerinit.InitPlayer(ctx, u.ID, database); err != nil {
				atomic.AddInt64(&failCount, 1)
				msg := strings.ToLower(err.Error())
				if strings.Contains(msg, "database is locked") || strings.Contains(msg, "busy") {
					atomic.AddInt64(&busyCount, 1)
				}
				return
			}

			lat := time.Since(latStart).Nanoseconds()
			latMu.Lock()
			latencies = append(latencies, lat)
			latMu.Unlock()
			atomic.AddInt64(&successCount, 1)
		}(i)
	}

	wg.Wait()
	elapsed := time.Since(start)

	return buildResult(concurrency, elapsed, &successCount, &failCount, &busyCount, &uniqueCount, latencies)
}

// runRawDBStress tests pure database write throughput (no bcrypt).
func runRawDBStress(t *testing.T, concurrency int, maxOpenConns int) *stressResult {
	ctx := context.Background()
	dbFile := fmt.Sprintf("_stress_raw_%d.db", concurrency)
	_ = os.Remove(dbFile)
	_ = os.Remove(dbFile + "-shm")
	_ = os.Remove(dbFile + "-wal")
	defer os.Remove(dbFile)
	defer os.Remove(dbFile + "-shm")
	defer os.Remove(dbFile + "-wal")

	database, err := db.Open(ctx, config.DB{
		URL:          fmt.Sprintf("file:%s?_pragma=foreign_keys(1)", dbFile),
		MaxOpenConns: int32(maxOpenConns),
		MaxIdleConns: int32(maxOpenConns),
		AutoMigrate:  true,
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer database.Close()

	var wg sync.WaitGroup
	var successCount, failCount, busyCount int64
	latencies := make([]int64, 0, concurrency)
	var errs []string
	var latMu sync.Mutex

	start := time.Now()
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			latStart := time.Now()

			var userID int64
			res, err := database.Conn.ExecContext(ctx,
				"INSERT INTO users (username, email, password_hash) VALUES (?, ?, ?)",
				fmt.Sprintf("raw%d", idx), fmt.Sprintf("raw%d@test.com", idx), "hash")
			if err != nil {
				atomic.AddInt64(&failCount, 1)
				if db.IsBusyError(err) {
					atomic.AddInt64(&busyCount, 1)
				} else {
					latMu.Lock()
					errs = append(errs, fmt.Sprintf("insert user: %v", err))
					latMu.Unlock()
				}
				return
			}
			userID, _ = res.LastInsertId()

			// Simulate playerinit: batch insert 8 skills in a tx
			err = database.InTx(ctx, func(q *dbgen.Queries) error {
				for sid := range gameconfig.AllSkillIDs() {
					if err := q.UpsertSkill(ctx, dbgen.UpsertSkillParams{
						UserID:  userID,
						SkillID: int64(sid),
						Level:   1,
						Xp:      0,
					}); err != nil {
						return err
					}
				}
				return nil
			})
			if err != nil {
				atomic.AddInt64(&failCount, 1)
				if db.IsBusyError(err) {
					atomic.AddInt64(&busyCount, 1)
				} else {
					latMu.Lock()
					errs = append(errs, fmt.Sprintf("tx: %v", err))
					latMu.Unlock()
				}
				return
			}

			lat := time.Since(latStart).Nanoseconds()
			latMu.Lock()
			latencies = append(latencies, lat)
			latMu.Unlock()
			atomic.AddInt64(&successCount, 1)
		}(i)
	}
	wg.Wait()
	elapsed := time.Since(start)

	return buildResult(concurrency, elapsed, &successCount, &failCount, &busyCount, new(int64), latencies)
}

func buildResult(concurrency int, elapsed time.Duration, successCount, failCount, busyCount, uniqueCount *int64, latencies []int64) *stressResult {
	success := atomic.LoadInt64(successCount)
	var minLat, maxLat, avgLat time.Duration
	var p95, p99 time.Duration
	if len(latencies) > 0 {
		minLat = time.Duration(latencies[0])
		maxLat = time.Duration(latencies[0])
		var total int64
		for _, v := range latencies {
			if v < minLat.Nanoseconds() {
				minLat = time.Duration(v)
			}
			if v > maxLat.Nanoseconds() {
				maxLat = time.Duration(v)
			}
			total += v
		}
		avgLat = time.Duration(total / int64(len(latencies)))
		// simple sort for p95/p99
		for i := 0; i < len(latencies); i++ {
			for j := i + 1; j < len(latencies); j++ {
				if latencies[i] > latencies[j] {
					latencies[i], latencies[j] = latencies[j], latencies[i]
				}
			}
		}
		p95Idx := int(float64(len(latencies)) * 0.95)
		p99Idx := int(float64(len(latencies)) * 0.99)
		if p95Idx >= len(latencies) {
			p95Idx = len(latencies) - 1
		}
		if p99Idx >= len(latencies) {
			p99Idx = len(latencies) - 1
		}
		p95 = time.Duration(latencies[p95Idx])
		p99 = time.Duration(latencies[p99Idx])
	}
	return &stressResult{
		concurrency:  concurrency,
		duration:     elapsed,
		success:      success,
		failures:     atomic.LoadInt64(failCount),
		busyErrors:   atomic.LoadInt64(busyCount),
		uniqueErrors: atomic.LoadInt64(uniqueCount),
		otherErrors:  atomic.LoadInt64(failCount) - atomic.LoadInt64(busyCount) - atomic.LoadInt64(uniqueCount),
		minLatency:   minLat,
		maxLatency:   maxLat,
		avgLatency:   avgLat,
		p95Latency:   p95,
		p99Latency:   p99,
	}
}

func logResult(t *testing.T, label string, r *stressResult) {
	t.Logf("=== %s ===", label)
	t.Logf("concurrency=%d duration=%v", r.concurrency, r.duration)
	t.Logf("success=%d fail=%d busy=%d unique=%d other=%d", r.success, r.failures, r.busyErrors, r.uniqueErrors, r.otherErrors)
	t.Logf("latency min=%v avg=%v p95=%v p99=%v max=%v", r.minLatency, r.avgLatency, r.p95Latency, r.p99Latency, r.maxLatency)
	if r.busyErrors > 0 {
		t.Errorf("unexpected busy errors: %d", r.busyErrors)
	}
}

func TestSQLiteConcurrency100_LowCost(t *testing.T)   { logResult(t, "100/low-cost", runStressWorkload(t, 100, 4, 4)) }
func TestSQLiteConcurrency500_LowCost(t *testing.T)   { logResult(t, "500/low-cost", runStressWorkload(t, 500, 4, 4)) }
func TestSQLiteConcurrency1000_LowCost(t *testing.T)  { logResult(t, "1000/low-cost", runStressWorkload(t, 1000, 4, 4)) }
func TestSQLiteConcurrency2000_LowCost(t *testing.T)  { logResult(t, "2000/low-cost", runStressWorkload(t, 2000, 4, 4)) }
func TestSQLiteConcurrency100_ProdCost(t *testing.T)  { logResult(t, "100/prod-cost", runStressWorkload(t, 100, 10, 4)) }
func TestSQLiteConcurrency1000_ProdCost(t *testing.T) { logResult(t, "1000/prod-cost", runStressWorkload(t, 1000, 10, 4)) }
func TestSQLiteConcurrency100_RawDB(t *testing.T)     { logResult(t, "100/raw-db", runRawDBStress(t, 100, 4)) }
func TestSQLiteConcurrency1000_RawDB(t *testing.T)    { logResult(t, "1000/raw-db", runRawDBStress(t, 1000, 4)) }
func TestSQLiteConcurrency5000_RawDB(t *testing.T)    { logResult(t, "5000/raw-db", runRawDBStress(t, 5000, 4)) }
