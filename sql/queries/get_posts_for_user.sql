-- name: GetPostsForUser :many
SELECT * FROM posts 
JOIN feeds on posts.feed_id = feeds.id
JOIN feed_follows on feed_follows.feed_id = feeds.id
JOIN users on feed_follows.user_id = users.id
WHERE users.name = $1
ORDER BY posts.created_at DESC LIMIT $2;