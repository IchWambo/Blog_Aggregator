-- name: GetFeeds :many
SELECT feeds.name, feeds.URL, users.name FROM feeds
JOIN users ON users.id = feeds.user_id;
