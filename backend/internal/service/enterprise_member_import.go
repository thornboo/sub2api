package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var (
	ErrEnterpriseMemberImportInvalid    = infraerrors.BadRequest("ENTERPRISE_MEMBER_IMPORT_INVALID", "enterprise member import file is invalid")
	ErrEnterpriseMemberImportExpired    = infraerrors.BadRequest("ENTERPRISE_MEMBER_IMPORT_PREVIEW_EXPIRED", "enterprise member import preview expired")
	ErrEnterpriseMemberImportConflict   = infraerrors.Conflict("ENTERPRISE_MEMBER_IMPORT_CONFLICT", "enterprise member import conflicts with current data")
	ErrEnterpriseMemberImportPending    = infraerrors.Conflict("ENTERPRISE_MEMBER_IMPORT_PENDING", "enterprise member import is not complete")
	ErrEnterpriseMemberImportConsumed   = infraerrors.Conflict("ENTERPRISE_MEMBER_IMPORT_RESULT_CONSUMED", "enterprise member import result secrets were already consumed")
	ErrEnterpriseMemberImportQueueEmpty = errors.New("enterprise member import queue is empty")
)

const (
	enterpriseMemberImportMaxFileBytes = 10 << 20
	enterpriseMemberImportMaxRows      = 5000
	enterpriseMemberImportMaxCellBytes = 4096
)

type EnterpriseMemberImportRow struct {
	RowNumber        int      `json:"row_number"`
	MemberCode       string   `json:"member_code"`
	MemberName       string   `json:"member_name"`
	MonthlyLimitUSD  float64  `json:"monthly_limit_usd"`
	RateLimit5h      float64  `json:"rate_limit_5h"`
	RateLimit1d      float64  `json:"rate_limit_1d"`
	RateLimit7d      float64  `json:"rate_limit_7d"`
	OpeningUsedUSD   float64  `json:"opening_used_usd"`
	KeyName          string   `json:"key_name,omitempty"`
	APIKeyCiphertext string   `json:"api_key_ciphertext,omitempty"`
	KeyPresent       bool     `json:"key_present"`
	KeyQuotaUSD      float64  `json:"key_quota_usd"`
	GroupIDs         []int64  `json:"group_ids"`
	Valid            bool     `json:"valid"`
	Errors           []string `json:"errors"`
	Warnings         []string `json:"warnings"`
}

type EnterpriseMemberImportPreview struct {
	JobID       int64                       `json:"job_id"`
	Token       string                      `json:"token,omitempty"`
	FileHash    string                      `json:"file_hash"`
	Format      string                      `json:"format"`
	ExpiresAt   time.Time                   `json:"expires_at"`
	Rows        []EnterpriseMemberImportRow `json:"rows"`
	ValidRows   int                         `json:"valid_rows"`
	InvalidRows int                         `json:"invalid_rows"`
}

type EnterpriseMemberImportJob struct {
	ID                      int64
	EnterpriseUserID        int64
	TokenHash               string
	FileHash                string
	Format                  string
	Status                  string
	Preview                 EnterpriseMemberImportPreview
	Result                  *EnterpriseMemberImportResult
	VersionFingerprint      map[string]int64
	IdempotencyKeyHash      *string
	ExpiresAt               time.Time
	CreatedAt               time.Time
	UpdatedAt               time.Time
	CompletedAt             *time.Time
	SelectedRows            []int
	QueuedAt                *time.Time
	StartedAt               *time.Time
	LockedAt                *time.Time
	LockOwner               *string
	AttemptCount            int
	ErrorCode               *string
	ErrorSummary            *string
	ResultSecretsConsumedAt *time.Time
}

type EnterpriseMemberImportQueueResult struct {
	JobID  int64  `json:"job_id"`
	Status string `json:"status"`
}

type EnterpriseMemberImportCreatedKey struct {
	MemberCode string `json:"member_code"`
	KeyName    string `json:"key_name"`
	Key        string `json:"key,omitempty"`
	KeyMasked  string `json:"key_masked"`
}

type EnterpriseMemberImportResult struct {
	JobID          int64                              `json:"job_id"`
	Status         string                             `json:"status"`
	CreatedMembers int                                `json:"created_members"`
	CreatedKeys    int                                `json:"created_keys"`
	Rows           []int                              `json:"rows"`
	Keys           []EnterpriseMemberImportCreatedKey `json:"keys,omitempty"`
	CompletedAt    time.Time                          `json:"completed_at"`
}

type EnterpriseMemberImportReferenceState struct {
	ExistingMemberCodes map[string]bool
	ExistingKeys        map[string]bool
	AuthorizedGroupIDs  map[int64]bool
	VersionFingerprint  map[string]int64
}

type EnterpriseMemberImportRepository interface {
	ValidateReferences(ctx context.Context, ownerID int64, memberCodes, keys []string, groupIDs []int64) (*EnterpriseMemberImportReferenceState, error)
	CreatePreviewJob(ctx context.Context, job *EnterpriseMemberImportJob) error
	GetPreviewJob(ctx context.Context, ownerID, jobID int64, tokenHash string) (*EnterpriseMemberImportJob, error)
	GetJobByToken(ctx context.Context, ownerID, jobID int64, tokenHash string) (*EnterpriseMemberImportJob, error)
	GetJob(ctx context.Context, ownerID, jobID int64) (*EnterpriseMemberImportJob, error)
	QueueCommit(ctx context.Context, ownerID, jobID int64, tokenHash string, selectedRows []int, idempotencyKeyHash string) (*EnterpriseMemberImportJob, error)
	ClaimNextCommitJob(ctx context.Context, workerID string, staleAfter time.Duration) (*EnterpriseMemberImportJob, error)
	RenewCommitLease(ctx context.Context, jobID int64, workerID string) (bool, error)
	Commit(ctx context.Context, job *EnterpriseMemberImportJob, rows []EnterpriseMemberImportRow, plaintextKeys map[int]string, idempotencyKeyHash, resultSecretsCiphertext string) (*EnterpriseMemberImportResult, error)
	MarkCommitFailed(ctx context.Context, jobID int64, workerID, errorCode, summary string) error
	ConsumeResultSecrets(ctx context.Context, ownerID, jobID int64, tokenHash string) (string, error)
	DeleteExpiredPreviews(ctx context.Context, limit int) (int64, error)
}

type EnterpriseMemberImportService struct {
	repo          EnterpriseMemberImportRepository
	encryptor     SecretEncryptor
	apiKeyService *APIKeyService
}

func NewEnterpriseMemberImportService(repo EnterpriseMemberImportRepository, encryptor SecretEncryptor, apiKeyService *APIKeyService) *EnterpriseMemberImportService {
	return &EnterpriseMemberImportService{repo: repo, encryptor: encryptor, apiKeyService: apiKeyService}
}

func (s *EnterpriseMemberImportService) Preview(ctx context.Context, ownerID int64, format string, data []byte) (result *EnterpriseMemberImportPreview, resultErr error) {
	startedAt := time.Now()
	defer func() {
		rows, invalidRows := 0, 0
		if result != nil {
			rows = len(result.Rows)
			invalidRows = result.InvalidRows
		}
		RecordEnterpriseMemberImportPreview(time.Since(startedAt), rows, invalidRows, resultErr)
	}()
	format = strings.ToLower(strings.TrimSpace(format))
	if ownerID <= 0 || len(data) == 0 || len(data) > enterpriseMemberImportMaxFileBytes || (format != "csv" && format != "xlsx") {
		return nil, ErrEnterpriseMemberImportInvalid
	}
	var rows []EnterpriseMemberImportRow
	var err error
	if format == "csv" {
		rows, err = parseEnterpriseMemberImportCSV(data)
	} else {
		rows, err = parseEnterpriseMemberImportXLSX(data)
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrEnterpriseMemberImportInvalid, err)
	}
	if len(rows) == 0 || len(rows) > enterpriseMemberImportMaxRows {
		return nil, ErrEnterpriseMemberImportInvalid
	}
	if err := s.encryptImportedKeys(rows); err != nil {
		return nil, err
	}
	validateEnterpriseMemberImportRows(rows)
	memberCodes, keys, groupIDs := enterpriseMemberImportReferenceValues(rows, s)
	references, err := s.repo.ValidateReferences(ctx, ownerID, memberCodes, keys, groupIDs)
	if err != nil {
		return nil, err
	}
	applyEnterpriseMemberImportReferenceErrors(rows, references, s)
	validRows := 0
	for i := range rows {
		rows[i].Valid = len(rows[i].Errors) == 0
		if rows[i].Valid {
			validRows++
		}
	}
	token, tokenHash, err := newEnterpriseMemberImportToken()
	if err != nil {
		return nil, err
	}
	fileDigest := sha256.Sum256(data)
	preview := EnterpriseMemberImportPreview{
		Token: token, FileHash: hex.EncodeToString(fileDigest[:]), Format: format,
		ExpiresAt: time.Now().Add(30 * time.Minute), Rows: rows, ValidRows: validRows, InvalidRows: len(rows) - validRows,
	}
	storedPreview := preview
	storedPreview.Token = ""
	job := &EnterpriseMemberImportJob{
		EnterpriseUserID: ownerID, TokenHash: tokenHash, FileHash: preview.FileHash, Format: format,
		Status: "previewed", Preview: storedPreview, VersionFingerprint: references.VersionFingerprint, ExpiresAt: preview.ExpiresAt,
	}
	if err := s.repo.CreatePreviewJob(ctx, job); err != nil {
		return nil, err
	}
	preview.JobID = job.ID
	publicPreview := preview
	publicPreview.Rows = append([]EnterpriseMemberImportRow(nil), preview.Rows...)
	for i := range publicPreview.Rows {
		publicPreview.Rows[i].APIKeyCiphertext = ""
	}
	return &publicPreview, nil
}

func (s *EnterpriseMemberImportService) Commit(ctx context.Context, ownerID, jobID int64, token string, selectedRows []int, idempotencyKey string) (result *EnterpriseMemberImportResult, resultErr error) {
	defer func() {
		rows := 0
		if result != nil {
			rows = len(result.Rows)
		}
		RecordEnterpriseMemberImportCommit(rows, resultErr)
	}()
	normalizedKey, err := NormalizeIdempotencyKey(idempotencyKey)
	if err != nil {
		return nil, err
	}
	job, err := s.repo.GetJobByToken(ctx, ownerID, jobID, hashEnterpriseMemberImportToken(token))
	if err != nil {
		return nil, err
	}
	if job.Status != "previewed" && job.Status != "queued" && job.Status != "processing" && job.Status != "completed" {
		return nil, ErrEnterpriseMemberImportConflict
	}
	if time.Now().After(job.ExpiresAt) {
		return nil, ErrEnterpriseMemberImportExpired
	}
	return s.processImportJob(ctx, job, selectedRows, HashIdempotencyKey(normalizedKey))
}

func (s *EnterpriseMemberImportService) QueueCommit(ctx context.Context, ownerID, jobID int64, token string, selectedRows []int, idempotencyKey string) (*EnterpriseMemberImportQueueResult, error) {
	normalizedKey, err := NormalizeIdempotencyKey(idempotencyKey)
	if err != nil || normalizedKey == "" {
		return nil, ErrEnterpriseMemberImportInvalid
	}
	job, err := s.repo.GetJobByToken(ctx, ownerID, jobID, hashEnterpriseMemberImportToken(token))
	if err != nil {
		return nil, err
	}
	if job.Status != "previewed" && job.Status != "queued" && job.Status != "processing" && job.Status != "completed" {
		return nil, ErrEnterpriseMemberImportConflict
	}
	if job.Status == "previewed" && time.Now().After(job.ExpiresAt) {
		return nil, ErrEnterpriseMemberImportExpired
	}
	normalizedRows, err := normalizeEnterpriseMemberImportSelection(job.Preview.Rows, selectedRows)
	if err != nil {
		return nil, err
	}
	queued, err := s.repo.QueueCommit(ctx, ownerID, jobID, hashEnterpriseMemberImportToken(token), normalizedRows, HashIdempotencyKey(normalizedKey))
	if err != nil {
		return nil, err
	}
	return &EnterpriseMemberImportQueueResult{JobID: queued.ID, Status: queued.Status}, nil
}

func normalizeEnterpriseMemberImportSelection(previewRows []EnterpriseMemberImportRow, selectedRows []int) ([]int, error) {
	valid := make(map[int]bool, len(previewRows))
	for _, row := range previewRows {
		if row.Valid {
			valid[row.RowNumber] = true
		}
	}
	selected := make(map[int]bool)
	if len(selectedRows) == 0 {
		for rowNumber := range valid {
			selected[rowNumber] = true
		}
	} else {
		for _, rowNumber := range selectedRows {
			if !valid[rowNumber] {
				return nil, ErrEnterpriseMemberImportInvalid
			}
			selected[rowNumber] = true
		}
	}
	if len(selected) == 0 {
		return nil, ErrEnterpriseMemberImportInvalid
	}
	out := make([]int, 0, len(selected))
	for rowNumber := range selected {
		out = append(out, rowNumber)
	}
	sort.Ints(out)
	return out, nil
}

func (s *EnterpriseMemberImportService) processImportJob(ctx context.Context, job *EnterpriseMemberImportJob, selectedRows []int, idempotencyKeyHash string) (*EnterpriseMemberImportResult, error) {
	if s == nil || s.repo == nil || job == nil || s.encryptor == nil || s.apiKeyService == nil {
		return nil, ErrEnterpriseMemberImportConflict
	}
	selected := make(map[int]bool)
	var err error
	for _, row := range selectedRows {
		selected[row] = true
	}
	rows := make([]EnterpriseMemberImportRow, 0, len(job.Preview.Rows))
	plaintextKeys := make(map[int]string)
	for _, row := range job.Preview.Rows {
		if !row.Valid || (len(selected) > 0 && !selected[row.RowNumber]) {
			continue
		}
		if row.KeyName != "" || row.KeyPresent {
			key := ""
			if row.APIKeyCiphertext != "" {
				key, err = s.encryptor.Decrypt(row.APIKeyCiphertext)
				if err != nil {
					return nil, ErrEnterpriseMemberImportConflict
				}
			} else {
				key, err = s.apiKeyService.GenerateKey()
				if err != nil {
					return nil, err
				}
			}
			plaintextKeys[row.RowNumber] = key
			row.KeyName = html.EscapeString(row.KeyName)
		}
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return nil, ErrEnterpriseMemberImportInvalid
	}
	secretRows := make([]EnterpriseMemberImportCreatedKey, 0, len(plaintextKeys))
	for _, row := range rows {
		if key := plaintextKeys[row.RowNumber]; key != "" {
			secretRows = append(secretRows, EnterpriseMemberImportCreatedKey{MemberCode: row.MemberCode, KeyName: row.KeyName, Key: key, KeyMasked: maskEnterpriseMemberImportKey(key)})
		}
	}
	secretsCiphertext := ""
	if len(secretRows) > 0 {
		secretJSON, marshalErr := json.Marshal(secretRows)
		if marshalErr != nil {
			return nil, marshalErr
		}
		secretsCiphertext, err = s.encryptor.Encrypt(string(secretJSON))
		if err != nil {
			return nil, err
		}
	}
	result, err := s.repo.Commit(ctx, job, rows, plaintextKeys, idempotencyKeyHash, secretsCiphertext)
	if err != nil {
		return nil, err
	}
	if s.apiKeyService != nil {
		s.apiKeyService.InvalidateAuthCacheByUserID(ctx, job.EnterpriseUserID)
	}
	return result, nil
}

func maskEnterpriseMemberImportKey(key string) string {
	if len(key) <= 12 {
		return "***"
	}
	return key[:6] + "..." + key[len(key)-4:]
}

func (s *EnterpriseMemberImportService) ProcessClaimedJob(ctx context.Context, job *EnterpriseMemberImportJob) (*EnterpriseMemberImportResult, error) {
	if job == nil || job.IdempotencyKeyHash == nil || len(job.SelectedRows) == 0 {
		return nil, ErrEnterpriseMemberImportConflict
	}
	result, err := s.processImportJob(ctx, job, job.SelectedRows, *job.IdempotencyKeyHash)
	RecordEnterpriseMemberImportCommit(len(job.SelectedRows), err)
	return result, err
}

func (s *EnterpriseMemberImportService) ConsumeResultSecrets(ctx context.Context, ownerID, jobID int64, token string) ([]EnterpriseMemberImportCreatedKey, error) {
	if strings.TrimSpace(token) == "" {
		return nil, ErrEnterpriseMemberImportInvalid
	}
	ciphertext, err := s.repo.ConsumeResultSecrets(ctx, ownerID, jobID, hashEnterpriseMemberImportToken(token))
	if err != nil {
		return nil, err
	}
	plaintext, err := s.encryptor.Decrypt(ciphertext)
	if err != nil {
		return nil, ErrEnterpriseMemberImportConflict
	}
	var keys []EnterpriseMemberImportCreatedKey
	if err := json.Unmarshal([]byte(plaintext), &keys); err != nil {
		return nil, ErrEnterpriseMemberImportConflict
	}
	return keys, nil
}

func (s *EnterpriseMemberImportService) GetJob(ctx context.Context, ownerID, jobID int64) (*EnterpriseMemberImportJob, error) {
	job, err := s.repo.GetJob(ctx, ownerID, jobID)
	if err != nil {
		return nil, err
	}
	job.Preview.Token = ""
	for i := range job.Preview.Rows {
		job.Preview.Rows[i].APIKeyCiphertext = ""
	}
	if job.Result != nil {
		for i := range job.Result.Keys {
			job.Result.Keys[i].Key = ""
		}
	}
	return job, nil
}

func (s *EnterpriseMemberImportService) encryptImportedKeys(rows []EnterpriseMemberImportRow) error {
	if s == nil || s.encryptor == nil || s.apiKeyService == nil {
		return errors.New("enterprise member import dependencies are unavailable")
	}
	seen := make(map[string]int)
	for i := range rows {
		plaintext := rows[i].APIKeyCiphertext
		rows[i].APIKeyCiphertext = ""
		if plaintext == "" {
			continue
		}
		if len(plaintext) > 128 || s.apiKeyService.ValidateCustomKey(plaintext) != nil {
			rows[i].Errors = append(rows[i].Errors, "invalid_api_key")
			continue
		}
		if previous, exists := seen[plaintext]; exists {
			rows[i].Errors = append(rows[i].Errors, fmt.Sprintf("duplicate_api_key_row_%d", previous))
		} else {
			seen[plaintext] = rows[i].RowNumber
		}
		ciphertext, err := s.encryptor.Encrypt(plaintext)
		if err != nil {
			return err
		}
		rows[i].APIKeyCiphertext = ciphertext
		rows[i].KeyPresent = true
	}
	return nil
}

func parseEnterpriseMemberImportCSV(data []byte) ([]EnterpriseMemberImportRow, error) {
	reader := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})))
	reader.ReuseRecord = false
	reader.FieldsPerRecord = -1
	headers, err := reader.Read()
	if err != nil {
		return nil, err
	}
	index := importHeaderIndex(headers)
	for _, required := range []string{"member_code", "member_name", "groups"} {
		if _, ok := index[required]; !ok {
			return nil, fmt.Errorf("missing column %s", required)
		}
	}
	rows := make([]EnterpriseMemberImportRow, 0)
	for rowNumber := 2; ; rowNumber++ {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		for _, cell := range record {
			if len(cell) > enterpriseMemberImportMaxCellBytes {
				return nil, errors.New("cell value too long")
			}
		}
		if importRecordEmpty(record) {
			continue
		}
		row := EnterpriseMemberImportRow{RowNumber: rowNumber, Errors: []string{}, Warnings: []string{}}
		row.MemberCode = importCell(record, index, "member_code")
		row.MemberName = importCell(record, index, "member_name")
		row.MonthlyLimitUSD, _ = parseImportAmount(importCell(record, index, "monthly_limit_usd"))
		row.RateLimit5h, _ = parseImportAmount(importCell(record, index, "rate_limit_5h"))
		row.RateLimit1d, _ = parseImportAmount(importCell(record, index, "rate_limit_1d"))
		row.RateLimit7d, _ = parseImportAmount(importCell(record, index, "rate_limit_7d"))
		row.OpeningUsedUSD, _ = parseImportAmount(importCell(record, index, "opening_used_usd"))
		row.KeyName = importCell(record, index, "key_name")
		row.APIKeyCiphertext = importCell(record, index, "api_key")
		row.KeyPresent = row.APIKeyCiphertext != "" || row.KeyName != ""
		row.KeyQuotaUSD, _ = parseImportAmount(importCell(record, index, "key_quota_usd"))
		row.GroupIDs, row.Errors = parseImportGroupIDs(importCell(record, index, "groups"), row.Errors)
		if _, err := parseImportAmount(importCell(record, index, "monthly_limit_usd")); err != nil {
			row.Errors = append(row.Errors, "invalid_monthly_limit")
		}
		for field, value := range map[string]string{
			"invalid_rate_limit_5h": importCell(record, index, "rate_limit_5h"),
			"invalid_rate_limit_1d": importCell(record, index, "rate_limit_1d"),
			"invalid_rate_limit_7d": importCell(record, index, "rate_limit_7d"),
		} {
			if _, err := parseImportAmount(value); err != nil {
				row.Errors = append(row.Errors, field)
			}
		}
		if _, err := parseImportAmount(importCell(record, index, "opening_used_usd")); err != nil {
			row.Errors = append(row.Errors, "invalid_opening_used")
		}
		if _, err := parseImportAmount(importCell(record, index, "key_quota_usd")); err != nil {
			row.Errors = append(row.Errors, "invalid_key_quota")
		}
		rows = append(rows, row)
		if len(rows) > enterpriseMemberImportMaxRows {
			return nil, errors.New("too many rows")
		}
	}
	return rows, nil
}

func validateEnterpriseMemberImportRows(rows []EnterpriseMemberImportRow) {
	type memberShape struct {
		name                                      string
		limit, limit5h, limit1d, limit7d, opening float64
		groups                                    string
		firstRow                                  int
	}
	members := make(map[string]memberShape)
	for i := range rows {
		row := &rows[i]
		row.MemberCode = strings.TrimSpace(row.MemberCode)
		row.MemberName = strings.TrimSpace(row.MemberName)
		row.KeyName = strings.TrimSpace(row.KeyName)
		if !enterpriseMemberCodePattern.MatchString(row.MemberCode) || len(row.MemberCode) > 100 {
			row.Errors = append(row.Errors, "invalid_member_code")
		}
		if row.MemberName == "" || len(row.MemberName) > 100 {
			row.Errors = append(row.Errors, "invalid_member_name")
		}
		if !validImportAmount(row.MonthlyLimitUSD) {
			row.Errors = append(row.Errors, "invalid_monthly_limit")
		}
		if !validImportAmount(row.RateLimit5h) {
			row.Errors = append(row.Errors, "invalid_rate_limit_5h")
		}
		if !validImportAmount(row.RateLimit1d) {
			row.Errors = append(row.Errors, "invalid_rate_limit_1d")
		}
		if !validImportAmount(row.RateLimit7d) {
			row.Errors = append(row.Errors, "invalid_rate_limit_7d")
		}
		if !validImportAmount(row.OpeningUsedUSD) {
			row.Errors = append(row.Errors, "invalid_opening_used")
		}
		if !validImportAmount(row.KeyQuotaUSD) {
			row.Errors = append(row.Errors, "invalid_key_quota")
		}
		if len(row.GroupIDs) == 0 {
			row.Errors = append(row.Errors, "groups_required")
		}
		if len(row.KeyName) > 100 {
			row.Errors = append(row.Errors, "invalid_key_name")
		}
		if row.KeyPresent && row.KeyName == "" {
			row.Errors = append(row.Errors, "key_name_required")
		}
		seenGroups := make(map[int64]bool, len(row.GroupIDs))
		for _, groupID := range row.GroupIDs {
			if seenGroups[groupID] {
				row.Errors = append(row.Errors, "duplicate_group")
			}
			seenGroups[groupID] = true
		}
		groupKey := fmt.Sprint(row.GroupIDs)
		if prior, exists := members[strings.ToLower(row.MemberCode)]; exists {
			if prior.name != row.MemberName || prior.limit != row.MonthlyLimitUSD || prior.limit5h != row.RateLimit5h || prior.limit1d != row.RateLimit1d || prior.limit7d != row.RateLimit7d || prior.groups != groupKey {
				row.Errors = append(row.Errors, "member_fields_conflict")
			}
			if row.OpeningUsedUSD != 0 {
				row.Errors = append(row.Errors, "opening_used_only_first_row")
			}
		} else {
			members[strings.ToLower(row.MemberCode)] = memberShape{row.MemberName, row.MonthlyLimitUSD, row.RateLimit5h, row.RateLimit1d, row.RateLimit7d, row.OpeningUsedUSD, groupKey, row.RowNumber}
			if row.MonthlyLimitUSD > 0 && row.OpeningUsedUSD >= row.MonthlyLimitUSD {
				row.Warnings = append(row.Warnings, "budget_exhausted_at_import")
			}
		}
	}
}

func enterpriseMemberImportReferenceValues(rows []EnterpriseMemberImportRow, s *EnterpriseMemberImportService) ([]string, []string, []int64) {
	memberSet, keySet, groupSet := map[string]bool{}, map[string]bool{}, map[int64]bool{}
	for i := range rows {
		memberSet[rows[i].MemberCode] = true
		for _, id := range rows[i].GroupIDs {
			groupSet[id] = true
		}
		if rows[i].APIKeyCiphertext != "" {
			if key, err := s.encryptor.Decrypt(rows[i].APIKeyCiphertext); err == nil {
				keySet[key] = true
			}
		}
	}
	members, keys, groups := make([]string, 0, len(memberSet)), make([]string, 0, len(keySet)), make([]int64, 0, len(groupSet))
	for value := range memberSet {
		members = append(members, value)
	}
	for value := range keySet {
		keys = append(keys, value)
	}
	for value := range groupSet {
		groups = append(groups, value)
	}
	return members, keys, groups
}

func applyEnterpriseMemberImportReferenceErrors(rows []EnterpriseMemberImportRow, state *EnterpriseMemberImportReferenceState, s *EnterpriseMemberImportService) {
	for i := range rows {
		row := &rows[i]
		if state.ExistingMemberCodes[strings.ToLower(row.MemberCode)] {
			row.Errors = append(row.Errors, "member_code_exists")
		}
		for _, id := range row.GroupIDs {
			if !state.AuthorizedGroupIDs[id] {
				row.Errors = append(row.Errors, fmt.Sprintf("group_%d_not_authorized", id))
			}
		}
		if row.APIKeyCiphertext != "" {
			if key, err := s.encryptor.Decrypt(row.APIKeyCiphertext); err != nil || state.ExistingKeys[key] {
				row.Errors = append(row.Errors, "api_key_exists")
			}
		}
	}
}

func importHeaderIndex(headers []string) map[string]int {
	index := make(map[string]int, len(headers))
	for i, header := range headers {
		index[strings.ToLower(strings.TrimSpace(header))] = i
	}
	return index
}
func importCell(record []string, index map[string]int, name string) string {
	i, ok := index[name]
	if !ok || i >= len(record) {
		return ""
	}
	return strings.TrimSpace(record[i])
}
func importRecordEmpty(record []string) bool {
	for _, value := range record {
		if strings.TrimSpace(value) != "" {
			return false
		}
	}
	return true
}
func parseImportAmount(value string) (float64, error) {
	if strings.TrimSpace(value) == "" {
		return 0, nil
	}
	amount, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil || !validImportAmount(amount) {
		return 0, errors.New("invalid amount")
	}
	return amount, nil
}
func validImportAmount(value float64) bool {
	return value >= 0 && value <= 99_999_999_999 && !math.IsNaN(value) && !math.IsInf(value, 0) && math.Abs(value*1e8-math.Round(value*1e8)) < 1e-5
}
func parseImportGroupIDs(value string, errs []string) ([]int64, []string) {
	parts := strings.FieldsFunc(value, func(r rune) bool { return r == '|' || r == ';' || r == ',' })
	ids, seen := make([]int64, 0, len(parts)), map[int64]bool{}
	for _, part := range parts {
		id, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64)
		if err != nil || id <= 0 {
			errs = append(errs, "invalid_group_identifier")
			continue
		}
		if seen[id] {
			errs = append(errs, "duplicate_group")
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	return ids, errs
}
func newEnterpriseMemberImportToken() (string, string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", err
	}
	token := hex.EncodeToString(raw)
	return token, hashEnterpriseMemberImportToken(token), nil
}
func hashEnterpriseMemberImportToken(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}

func EnterpriseMemberImportCSVTemplate() []byte {
	return []byte("member_code,member_name,rate_limit_5h,rate_limit_1d,rate_limit_7d,monthly_limit_usd,opening_used_usd,key_name,api_key,key_quota_usd,groups\nemployee-001,Example Member,25,50,75,100,0,Primary Key,,0,1|2\n")
}

type EnterpriseMemberImportCleanupService struct {
	repo   EnterpriseMemberImportRepository
	cancel context.CancelFunc
}

func NewEnterpriseMemberImportCleanupService(repo EnterpriseMemberImportRepository) *EnterpriseMemberImportCleanupService {
	return &EnterpriseMemberImportCleanupService{repo: repo}
}

func (s *EnterpriseMemberImportCleanupService) Start() {
	if s == nil || s.repo == nil || s.cancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for {
			cleanupCtx, cleanupCancel := context.WithTimeout(ctx, 30*time.Second)
			_, _ = s.repo.DeleteExpiredPreviews(cleanupCtx, 500)
			cleanupCancel()
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

func (s *EnterpriseMemberImportCleanupService) Stop() {
	if s != nil && s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
}

func ProvideEnterpriseMemberImportCleanupService(repo EnterpriseMemberImportRepository) *EnterpriseMemberImportCleanupService {
	cleanup := NewEnterpriseMemberImportCleanupService(repo)
	cleanup.Start()
	return cleanup
}
