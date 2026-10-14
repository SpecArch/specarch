-- name: AddShelf :one
INSERT INTO shelf (code, label)
VALUES ($1, $2)
RETURNING code, label;
