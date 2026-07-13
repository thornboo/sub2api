-- Enterprise owner error history is commonly filtered by owner/member and time.
-- Non-transactional migration: CREATE INDEX CONCURRENTLY cannot run in a transaction.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ops_error_logs_user_member_time
  ON ops_error_logs (user_id, member_id, created_at DESC)
  WHERE user_id IS NOT NULL AND member_id IS NOT NULL;
