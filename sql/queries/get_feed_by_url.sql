-- name: GetFeedByUrl :one
SELECT * FROM feeds WHERE url LIKE $1;