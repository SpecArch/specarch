-- name: RegisterMember :one
INSERT INTO members (card_number, address_city, address_postcode, address_street, full_name)
VALUES ($1, $2, $3, $4, $5)
RETURNING card_number, address_city, address_postcode, address_street, full_name;
