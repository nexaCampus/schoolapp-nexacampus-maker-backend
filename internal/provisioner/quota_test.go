package provisioner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/nexaCampus/schoolapp-nexacampus-maker-backend/internal/database"
)

func setupTestDB(t *testing.T) string {
	tempDir, err := os.MkdirTemp("", "quota-test")
	if err != nil {
		t.Fatalf("creating temp dir: %v", err)
	}
	dbPath := filepath.Join(tempDir, "master.db")
	db, err := database.OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("opening sqlite: %v", err)
	}
	defer db.Close()

	if err := database.MigrateMaster(db); err != nil {
		t.Fatalf("migrating master db: %v", err)
	}
	return dbPath
}

func TestSingleUseFreeTierQuota(t *testing.T) {
	dbPath := setupTestDB(t)
	defer os.RemoveAll(filepath.Dir(dbPath))

	db, err := database.OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("reopening db: %v", err)
	}
	defer db.Close()

	qm := NewQuotaManager(db)
	ctx := context.Background()

	// 1. Initial State: Free tier must be available
	avail, err := qm.IsFreeTierAvailable(ctx)
	if err != nil {
		t.Fatalf("checking free tier: %v", err)
	}
	if !avail {
		t.Fatalf("expected free tier to be initially available, got false")
	}

	opts, err := qm.GetDeploymentOptions(ctx)
	if err != nil {
		t.Fatalf("getting deployment options: %v", err)
	}
	if !opts.FreeTierAvailable || opts.FreeTierConsumed {
		t.Errorf("expected FreeTierAvailable=true and FreeTierConsumed=false, got %+v", opts)
	}

	// Verify free plan is present in options list
	hasFreePlan := false
	for _, p := range opts.AvailablePlans {
		if p.ID == "free" {
			hasFreePlan = true
			break
		}
	}
	if !hasFreePlan {
		t.Errorf("expected 'free' plan in available plans list")
	}

	// 2. Consume Free Tier for first school ('lcgvm')
	err = qm.ConsumeFreeTierAtomic(ctx, "lcgvm")
	if err != nil {
		t.Fatalf("first consume should succeed, got %v", err)
	}

	// 3. Second school ('dps') attempts to claim free tier -> MUST FAIL with ErrFreeTierExhausted
	err = qm.ConsumeFreeTierAtomic(ctx, "dps-delhi")
	if !errors.Is(err, ErrFreeTierExhausted) {
		t.Fatalf("second consume should fail with ErrFreeTierExhausted, got %v", err)
	}

	// 4. Verify Free Tier Suppression in API options
	availAfter, err := qm.IsFreeTierAvailable(ctx)
	if err != nil {
		t.Fatalf("checking free tier: %v", err)
	}
	if availAfter {
		t.Fatalf("expected free tier to be consumed (false), got true")
	}

	optsAfter, err := qm.GetDeploymentOptions(ctx)
	if err != nil {
		t.Fatalf("getting deployment options after consumption: %v", err)
	}
	if optsAfter.FreeTierAvailable || !optsAfter.FreeTierConsumed {
		t.Errorf("expected FreeTierAvailable=false, FreeTierConsumed=true, got %+v", optsAfter)
	}

	// Verify 'free' plan is completely suppressed
	for _, p := range optsAfter.AvailablePlans {
		if p.ID == "free" {
			t.Errorf("free tier plan was NOT suppressed from available plans!")
		}
	}
}
