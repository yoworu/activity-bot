-- name: GetFandom :one
SELECT *
FROM fandoms
WHERE chat_id = $1
  AND name = $2;

-- name: DeleteFandom :exec
DELETE FROM fandoms WHERE chat_id = $1 AND name = $2;

-- name: GetOrCreateFandom :one
INSERT INTO fandoms (chat_id,
                     name)
VALUES ($1, $2)
ON CONFLICT (chat_id, name)
    DO UPDATE SET name = EXCLUDED.name
RETURNING
    id,
    chat_id,
    name;


-- name: CreateRoleCategory :one
INSERT INTO role_categories (fandom_id,
                             name)
VALUES ($1, $2)
ON CONFLICT (fandom_id, name)
    DO UPDATE SET name = EXCLUDED.name
RETURNING
    id,
    fandom_id,
    name,
    created_at;


-- name: CreateRole :one
INSERT INTO roles (category_id,
                   name,
                   emoji)
VALUES ($1, $2, $3)
ON CONFLICT (category_id, name)
    DO UPDATE SET emoji = EXCLUDED.emoji
RETURNING
    id,
    category_id,
    name,
    emoji,
    created_at;


-- name: CreateRoleAlias :one
INSERT INTO role_aliases (role_id,
                          name)
VALUES ($1, $2)
ON CONFLICT (role_id, name)
    DO NOTHING
RETURNING
    id,
    role_id,
    name;


-- name: GetRoleByName :one
SELECT r.*
FROM roles r
         JOIN role_categories rc ON rc.id = r.category_id
         JOIN fandoms f ON f.id = rc.fandom_id
WHERE f.chat_id = $1
  AND f.name = $2
  AND rc.name = $3
  AND r.name = $4;


-- name: GetRoleByAlias :one
SELECT r.*
FROM roles r
         JOIN role_aliases ra ON ra.role_id = r.id
         JOIN role_categories rc ON rc.id = r.category_id
         JOIN fandoms f ON f.id = rc.fandom_id
WHERE f.chat_id = $1
  AND f.name = $2
  AND ra.name = $3;


-- name: GetRoleByNameOrAlias :one
SELECT r.*
FROM roles r
         JOIN role_categories rc ON rc.id = r.category_id
         JOIN fandoms f ON f.id = rc.fandom_id
         LEFT JOIN role_aliases ra ON ra.role_id = r.id
WHERE f.chat_id = $1
  AND LOWER(f.name) = LOWER($2)
  AND (
    LOWER(r.name) = LOWER($3)
        OR LOWER(ra.name) = LOWER($3)
    )
LIMIT 1;


-- name: CreateRoleReservation :exec
INSERT INTO role_reservations (chat_id,
                               user_id,
                               role_id)
VALUES ($1, $2, $3)
ON CONFLICT (chat_id, role_id)
    DO NOTHING;

-- name: DeleteRoleReservation :exec
DELETE
FROM role_reservations
WHERE chat_id = $1
  AND role_id = $2;


-- name: GetRoleReservation :one
SELECT *
FROM role_reservations
WHERE chat_id = $1
  AND role_id = $2;


-- name: ListRoleReservations :many
SELECT sqlc.embed(rr),
       sqlc.embed(r),
       sqlc.embed(rc),
       sqlc.embed(f)
FROM role_reservations rr
         JOIN roles r ON r.id = rr.role_id
         JOIN role_categories rc ON rc.id = r.category_id
         JOIN fandoms f ON f.id = rc.fandom_id
WHERE rr.chat_id = $1
ORDER BY f.name, rc.name, r.name;


-- name: GetRoleCategory :one
SELECT id,
       fandom_id,
       name,
       created_at
FROM role_categories
WHERE fandom_id = $1
  AND name = $2;


-- name: ListRoleCategories :many
SELECT id,
       fandom_id,
       name,
       created_at
FROM role_categories
WHERE fandom_id = $1
ORDER BY name;

-- name: GetFandomWithRoles :many
SELECT f.id          AS fandom_id,
       f.chat_id     AS fandom_chat_id,
       f.name        AS fandom_name,

       rc.id         AS category_id,
       rc.fandom_id  AS category_fandom_id,
       rc.name       AS category_name,
       rc.created_at AS category_created_at,

       r.id          AS role_id,
       r.category_id AS role_category_id,
       r.name        AS role_name,
       r.emoji       AS role_emoji,
       r.created_at  AS role_created_at,

       ra.id         AS alias_id,
       ra.role_id    AS alias_role_id,
       ra.name       AS alias_name
FROM fandoms f
         LEFT JOIN role_categories rc
                   ON rc.fandom_id = f.id
         LEFT JOIN roles r
                   ON r.category_id = rc.id
         LEFT JOIN role_aliases ra
                   ON ra.role_id = r.id
WHERE f.chat_id = $1
  AND f.name = $2
ORDER BY rc.created_at,
         r.name,
         ra.name;


-- name: ListRoleTemplates :many
SELECT f.id          AS fandom_id,
       f.chat_id     AS fandom_chat_id,
       f.name        AS fandom_name,

       rc.id         AS category_id,
       rc.fandom_id  AS category_fandom_id,
       rc.name       AS category_name,
       rc.created_at AS category_created_at,

       r.id          AS role_id,
       r.category_id AS role_category_id,
       r.name        AS role_name,
       r.emoji       AS role_emoji,
       r.created_at  AS role_created_at,

       ra.id         AS alias_id,
       ra.role_id    AS alias_role_id,
       ra.name       AS alias_name
FROM fandoms f
         JOIN role_categories rc
              ON rc.fandom_id = f.id
         JOIN roles r
              ON r.category_id = rc.id
         LEFT JOIN role_aliases ra
                   ON ra.role_id = r.id
WHERE f.chat_id = $1
ORDER BY f.name,
         rc.id,
         r.name,
         ra.name;