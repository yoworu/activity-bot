-- name: GetChatMember :one
SELECT sqlc.embed(chat_members), sqlc.embed(users), sqlc.embed(chats)
FROM chat_members
         JOIN users ON users.id = user_id
         JOIN chats ON chat_members.chat_id = chats.id AND chat_id = $1 AND user_id = $2;

-- name: CreateChatMember :exec
INSERT INTO chat_members(chat_id, user_id, tag, status, rest_until, left_at, rest_reason)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: SetChatMemberTag :exec
UPDATE chat_members
SET tag = $1
WHERE user_id = $2
  AND chat_id = $3;

-- name: MarkChatMemberLeft :exec
UPDATE chat_members
SET left_at = $1
WHERE user_id = $2
  AND chat_id = $3;

-- name: MarkAllChatMembersLeftExcept :exec
UPDATE chat_members
SET left_at = @left_at
WHERE chat_id = @chat_id
  AND left_at IS NULL
  AND user_id <> ALL (@user_ids::BIGINT[]);

-- name: UpsertChatMembersAndUsers :exec
WITH upserted_users AS (
    INSERT INTO users (
                       id,
                       username,
                       first_name,
                       last_name,
                       is_bot,
                       emoji
        )
        SELECT u.id,
               u.username,
               u.first_name,
               u.last_name,
               COALESCE(u.is_bot, false),
               emoji
        FROM ROWS FROM (
                 UNNEST(@user_ids::BIGINT[]),
                 UNNEST(@usernames::TEXT[]),
                 UNNEST(@first_names::TEXT[]),
                 UNNEST(@last_names::TEXT[]),
                 UNNEST(@is_bots::BOOLEAN[]),
                 UNNEST(@emojis::TEXT[])
                 ) AS u(id, username, first_name, last_name, is_bot, emoji)
        ON CONFLICT (id) DO UPDATE SET
            username = EXCLUDED.username,
            first_name = EXCLUDED.first_name,
            last_name = EXCLUDED.last_name,
            is_bot = EXCLUDED.is_bot,
            emoji = CASE
                        WHEN (users.emoji IS NULL OR users.emoji = '')
                            AND EXCLUDED.emoji <> ''
                            THEN EXCLUDED.emoji
                        ELSE users.emoji
                END
        RETURNING id)
INSERT
INTO chat_members (chat_id, user_id, tag, status)
SELECT @chat_id,
       m.user_id,
       m.tag,
       m.status
FROM ROWS FROM (
         UNNEST(@user_ids::BIGINT[]),
         UNNEST(@tags::TEXT[]),
         UNNEST(@statuses::SMALLINT[])
         ) AS m(user_id, tag, status)
         JOIN upserted_users AS uu ON m.user_id = uu.id
ON CONFLICT (chat_id, user_id)
    DO UPDATE
    SET tag     = CASE
                      WHEN EXCLUDED.tag <> '' THEN EXCLUDED.tag
                      ELSE chat_members.tag
        END,
        status  = GREATEST(EXCLUDED.status, chat_members.status),
        left_at = CASE
                      WHEN chat_members.left_at IS NOT NULL THEN NULL
                      ELSE chat_members.left_at
            END
;

-- name: ListChatMembers :many
SELECT sqlc.embed(cm), sqlc.embed(u)
FROM chat_members cm
         JOIN users u ON u.id = cm.user_id
WHERE cm.chat_id = @chat_id
  AND (
    sqlc.narg(is_bot)::boolean IS NULL
        OR u.is_bot = sqlc.narg(is_bot)
    )
  AND (
    sqlc.narg(has_left)::boolean IS NULL
        OR (cm.left_at IS NOT NULL) = sqlc.narg(has_left)
    )
  AND (
    sqlc.narg(exclude_from_call)::boolean IS NULL
        OR cm.exclude_from_call = sqlc.narg(exclude_from_call)
    )
ORDER BY cm.joined_at;

-- name: GetChatMemberByUsername :one
SELECT sqlc.embed(cm), sqlc.embed(u)
FROM chat_members cm
         JOIN users u ON u.id = cm.user_id
WHERE cm.chat_id = $1
  AND u.username ILIKE $2
LIMIT 1;

-- name: SetChatMemberExcludeFromSummon :exec
UPDATE chat_members
SET exclude_from_call = $1
WHERE user_id = $2
  AND chat_id = $3;

-- name: ListChatAdmins :many
SELECT sqlc.embed(cm), sqlc.embed(u)
FROM chat_members cm
         JOIN users u ON u.id = cm.user_id
WHERE cm.chat_id = $1
  AND is_bot = false
  AND cm.status >= $2
  AND cm.left_at IS NULL
ORDER BY cm.status DESC;

-- name: SetChatMemberDescription :exec
UPDATE chat_members
SET description = $1
WHERE chat_id = $2
  AND user_id = $3;

-- name: SetChatMemberBirthday :exec
UPDATE chat_members
SET birthday = $1
WHERE chat_id = $2
  AND user_id = $3;