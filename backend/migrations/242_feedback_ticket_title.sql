ALTER TABLE feedbacks
    ADD COLUMN IF NOT EXISTS title TEXT NOT NULL DEFAULT '';

UPDATE feedbacks
SET title = left(regexp_replace(btrim(content), '\s+', ' ', 'g'), 120)
WHERE btrim(title) = '';

ALTER TABLE feedbacks
    DROP CONSTRAINT IF EXISTS feedbacks_title_length_check;

ALTER TABLE feedbacks
    ADD CONSTRAINT feedbacks_title_length_check CHECK (char_length(title) <= 120);

COMMENT ON COLUMN feedbacks.title IS 'Normalized single-line ticket title. Legacy blank titles may be rendered from content by the application.';
