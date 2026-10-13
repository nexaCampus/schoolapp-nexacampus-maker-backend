package provisioner

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/nexaCampus/schoolapp-nexacampus-maker-backend/internal/models"
)

var (
	ErrFreeTierExhausted = errors.New("free tier quota reached on master account; please select a starter plan or bring your own cloud api key")
)

const (
	MetaKeyFreeTierConsumed = "platform_free_tier_consumed"
)

// QuotaManager governs the platform's cloud quotas, ensuring the single-use free tier
// cannot be over-subscribed or raced by multiple school creation requests.
type QuotaManager struct {
	db *sql.DB
}

// NewQuotaManager constructs a quota controller backed by the master SQLite database.
func NewQuotaManager(db *sql.DB) *QuotaManager {
	return &QuotaManager{db: db}
}

// IsFreeTierAvailable checks whether the single platform free tier slot remains unclaimed.
func (qm *QuotaManager) IsFreeTierAvailable(ctx context.Context) (bool, error) {
	var val string
	query := `SELECT value FROM platform_meta WHERE key = ?`
	err := qm.db.QueryRowContext(ctx, query, MetaKeyFreeTierConsumed).Scan(&val)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// If not set, initialize to false
			_, _ = qm.db.ExecContext(ctx, `INSERT OR IGNORE INTO platform_meta (key, value) VALUES (?, 'false')`, MetaKeyFreeTierConsumed)
			return true, nil
		}
		return false, fmt.Errorf("reading free tier quota: %w", err)
	}

	return strings.ToLower(val) == "false", nil
}

// ConsumeFreeTierAtomic checks and locks the free tier flag inside an atomic transaction.
// If another concurrent request or prior tenant already consumed it, returns ErrFreeTierExhausted.
func (qm *QuotaManager) ConsumeFreeTierAtomic(ctx context.Context, tenantSlug string) error {
	tx, err := qm.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return fmt.Errorf("beginning atomic quota transaction: %w", err)
	}
	defer tx.Rollback()

	var val string
	query := `SELECT value FROM platform_meta WHERE key = ?`
	err = tx.QueryRowContext(ctx, query, MetaKeyFreeTierConsumed).Scan(&val)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			val = "false"
		} else {
			return fmt.Errorf("reading quota in transaction: %w", err)
		}
	}

	if strings.ToLower(val) == "true" {
		return ErrFreeTierExhausted
	}

	// Mark consumed
	updateQuery := `INSERT INTO platform_meta (key, value) VALUES (?, 'true') ON CONFLICT(key) DO UPDATE SET value = 'true'`
	if _, err := tx.ExecContext(ctx, updateQuery, MetaKeyFreeTierConsumed); err != nil {
		return fmt.Errorf("updating free tier quota: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing quota transaction: %w", err)
	}

	return nil
}

// GetDeploymentOptions produces the deployment tiers, suppressing the free tier once consumed.
func (qm *QuotaManager) GetDeploymentOptions(ctx context.Context) (*models.DeploymentOptionsResponse, error) {
	freeAvailable, err := qm.IsFreeTierAvailable(ctx)
	if err != nil {
		return nil, err
	}

	plans := []models.Plan{
		{
			ID:          "starter",
			Name:        "Render Starter (512 MB)",
			Description: "Dedicated production web service with high availability and persistent storage",
			RAMBudget:   "512 MB",
			Available:   true,
			RequiresBYO: false,
		},
		{
			ID:          "byo",
			Name:        "Bring Your Own Account (BYO)",
			Description: "Deploy to your personal Render, Vercel, or AWS workspace with your API key",
			RAMBudget:   "Unlimited / Custom",
			Available:   true,
			RequiresBYO: true,
		},
	}

	if freeAvailable {
		freePlan := models.Plan{
			ID:          "free",
			Name:        "Render Community Free Tier (Single-Use)",
			Description: "Promotional zero-cost tier (limited to exactly 1 active school per platform account)",
			RAMBudget:   "512 MB Shared",
			Available:   true,
			RequiresBYO: false,
		}
		// Prepend free plan
		plans = append([]models.Plan{freePlan}, plans...)
	}

	return &models.DeploymentOptionsResponse{
		FreeTierAvailable: freeAvailable,
		FreeTierConsumed:  !freeAvailable,
		ActiveProviders:   []string{"render", "vercel", "aws", "local"},
		AvailablePlans:    plans,
		Message: func() string {
			if freeAvailable {
				return "Promotional single-use free tier is active and ready to claim."
			}
			return "Free tier quota has been claimed. Showing starter and BYO cloud plans."
		}(),
	}, nil
}
