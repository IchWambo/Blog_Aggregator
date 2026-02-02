-- name: GetFeedFollowsForUsers :many
SELECT feed_follows.*, users.name, feeds.name FROM feed_follows 
INNER JOIN users on users.id = feed_follows.user_id
INNER JOIN feeds on feeds.id = feed_follows.feed_id
WHERE users.name LIKE $1;