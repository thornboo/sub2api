-- Build the composite API-key identity required by the following transactional
-- integrity migration without taking a long blocking index lock in production.

CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_api_keys_id_member_owner
    ON api_keys(id, member_id, user_id);
