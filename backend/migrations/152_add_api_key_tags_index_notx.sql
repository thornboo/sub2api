-- Keep tag filters index-backed without holding a long transaction.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_api_keys_tags_gin
  ON api_keys USING GIN (tags)
  WHERE deleted_at IS NULL;
