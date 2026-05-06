// Package playerinit runs one-time initialization for newly created players.
// It is called from auth.Service.Register after the users row is inserted.
package playerinit

import (
	"context"
	"fmt"

	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/db"
	dbgen "github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/db/gen"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/gameconfig"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/skill"
)

// InitPlayer initializes a brand-new player in the database.
// It runs inside a single transaction so partial failures roll back cleanly.
// Currently it seeds every skill at level 1; additional starter data
// (inventory, unlocked events, equipment) can be added here later.
func InitPlayer(ctx context.Context, userID int64, database *db.DB) error {
	curve, err := skill.LoadCurve()
	if err != nil {
		return fmt.Errorf("playerinit: load skill curve: %w", err)
	}

	xpForLevel1 := curve.XPForLevel(1)

	return database.InTx(ctx, func(q *dbgen.Queries) error {
		for _, sid := range gameconfig.AllSkillIDs() {
			if err := q.UpsertSkill(ctx, dbgen.UpsertSkillParams{
				UserID:  userID,
				SkillID: int64(sid),
				Level:   1,
				Xp:      xpForLevel1,
			}); err != nil {
				return fmt.Errorf("playerinit: upsert skill %d: %w", sid, err)
			}
		}
		if err := q.MarkPlayerInit(ctx, userID); err != nil {
			return fmt.Errorf("playerinit: mark initialized: %w", err)
		}
		return nil
	})
}
