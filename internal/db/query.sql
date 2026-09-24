-- name: ListRepos :many
SELECT * FROM repos
ORDER BY name;

-- name: ListActiveRepos :many
SELECT * FROM repos
WHERE active = TRUE
ORDER BY name;

-- name: GetRepo :one
SELECT * FROM repos
WHERE id = $1 LIMIT 1;

-- name: GetRepoByName :one
SELECT * FROM repos
WHERE name = $1 LIMIT 1;

-- name: CreateRepo :one
INSERT INTO repos (name, url, owner, tags, active)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: UpdateRepo :one
UPDATE repos
SET
    url = coalesce(sqlc.narg('url'), url),
    owner = coalesce(sqlc.narg('owner'), owner),
    tags = coalesce(sqlc.narg('tags'), tags),
    active = coalesce(sqlc.narg('active'), active),
    updated_at = now()
WHERE id = sqlc.arg('id')
RETURNING *;

-- name: DeleteRepo :exec
DELETE FROM repos
WHERE id = $1;
