-- name: LoadSkills :many
SELECT user_id, skill_id, level, xp
FROM player_skills
WHERE user_id = ?
ORDER BY skill_id;

-- name: UpsertSkill :exec
INSERT INTO player_skills (user_id, skill_id, level, xp, updated_at)
VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
ON CONFLICT(user_id, skill_id) DO UPDATE SET
    level = excluded.level,
    xp    = excluded.xp,
    updated_at = CURRENT_TIMESTAMP
WHERE level IS NOT excluded.level OR xp IS NOT excluded.xp;
