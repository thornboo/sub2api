package service

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type importPreviewRepoCapture struct{ job *EnterpriseMemberImportJob }

func (r *importPreviewRepoCapture) ValidateReferences(context.Context, int64, []string, []string, []int64) (*EnterpriseMemberImportReferenceState, error) {
	return &EnterpriseMemberImportReferenceState{ExistingMemberCodes: map[string]bool{}, ExistingKeys: map[string]bool{}, AuthorizedGroupIDs: map[int64]bool{1: true}, VersionFingerprint: map[string]int64{"group:1": 1}}, nil
}
func (r *importPreviewRepoCapture) CreatePreviewJob(_ context.Context, job *EnterpriseMemberImportJob) error {
	copied := *job
	copied.ID = 9
	r.job = &copied
	job.ID = 9
	return nil
}
func (r *importPreviewRepoCapture) GetPreviewJob(context.Context, int64, int64, string) (*EnterpriseMemberImportJob, error) {
	panic("unexpected")
}
func (r *importPreviewRepoCapture) GetJob(context.Context, int64, int64) (*EnterpriseMemberImportJob, error) {
	panic("unexpected")
}
func (r *importPreviewRepoCapture) GetJobByToken(context.Context, int64, int64, string) (*EnterpriseMemberImportJob, error) {
	panic("unexpected")
}
func (r *importPreviewRepoCapture) QueueCommit(context.Context, int64, int64, string, []int, string) (*EnterpriseMemberImportJob, error) {
	panic("unexpected")
}
func (r *importPreviewRepoCapture) ClaimNextCommitJob(context.Context, string, time.Duration) (*EnterpriseMemberImportJob, error) {
	return nil, ErrEnterpriseMemberImportQueueEmpty
}
func (r *importPreviewRepoCapture) RenewCommitLease(context.Context, int64, string) (bool, error) {
	return true, nil
}
func (r *importPreviewRepoCapture) Commit(context.Context, *EnterpriseMemberImportJob, []EnterpriseMemberImportRow, map[int]string, string, string) (*EnterpriseMemberImportResult, error) {
	panic("unexpected")
}
func (r *importPreviewRepoCapture) MarkCommitFailed(context.Context, int64, string, string, string) error {
	return nil
}
func (r *importPreviewRepoCapture) ConsumeResultSecrets(context.Context, int64, int64, string) (string, error) {
	panic("unexpected")
}
func (r *importPreviewRepoCapture) DeleteExpiredPreviews(context.Context, int) (int64, error) {
	return 0, nil
}

type importTestEncryptor struct{}

func (importTestEncryptor) Encrypt(value string) (string, error) { return "encrypted:" + value, nil }
func (importTestEncryptor) Decrypt(value string) (string, error) {
	return strings.TrimPrefix(value, "encrypted:"), nil
}

type importQueueRepo struct {
	importPreviewRepoCapture
	job             *EnterpriseMemberImportJob
	queuedRows      []int
	queuedTokenHash string
	queuedKeyHash   string
}

func (r *importQueueRepo) GetJobByToken(_ context.Context, ownerID, jobID int64, tokenHash string) (*EnterpriseMemberImportJob, error) {
	if r.job == nil || r.job.EnterpriseUserID != ownerID || r.job.ID != jobID || r.job.TokenHash != tokenHash {
		return nil, ErrEnterpriseMemberImportExpired
	}
	return r.job, nil
}

func (r *importQueueRepo) QueueCommit(_ context.Context, ownerID, jobID int64, tokenHash string, selectedRows []int, idempotencyKeyHash string) (*EnterpriseMemberImportJob, error) {
	r.queuedRows = append([]int(nil), selectedRows...)
	r.queuedTokenHash = tokenHash
	r.queuedKeyHash = idempotencyKeyHash
	queued := *r.job
	queued.Status = "queued"
	return &queued, nil
}

func TestParseEnterpriseMemberImportCSVPreservesOrderedGroups(t *testing.T) {
	data := []byte("member_code,member_name,rate_limit_5h,rate_limit_1d,rate_limit_7d,monthly_limit_usd,opening_used_usd,key_name,api_key,key_quota_usd,groups\nmember-1,Alice,25,50,75,100.25,12.5,Primary,abcdefghijklmnop,20,3|1|7\n")
	rows, err := parseEnterpriseMemberImportCSV(data)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, []int64{3, 1, 7}, rows[0].GroupIDs)
	require.Equal(t, 100.25, rows[0].MonthlyLimitUSD)
	require.Equal(t, 25.0, rows[0].RateLimit5h)
	require.Equal(t, 50.0, rows[0].RateLimit1d)
	require.Equal(t, 75.0, rows[0].RateLimit7d)
	require.Equal(t, 12.5, rows[0].OpeningUsedUSD)
	require.Equal(t, "abcdefghijklmnop", rows[0].APIKeyCiphertext)
}

func TestEnterpriseMemberImportCSVTemplateUsesChineseHeadersAndRoundTrips(t *testing.T) {
	template := EnterpriseMemberImportCSVTemplate()
	require.True(t, bytes.HasPrefix(template, []byte{0xEF, 0xBB, 0xBF}))
	require.Contains(t, string(template), "成员编号,成员名称,5小时限额")

	rows, err := parseEnterpriseMemberImportCSV(template)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "employee-001", rows[0].MemberCode)
	require.Equal(t, "示例成员", rows[0].MemberName)
	require.Equal(t, "主密钥", rows[0].KeyName)
	require.Equal(t, []int64{1, 2}, rows[0].GroupIDs)
}

func TestParseEnterpriseMemberImportCSVEnforces5000RowCapacityBoundary(t *testing.T) {
	rows, err := parseEnterpriseMemberImportCSV(buildEnterpriseMemberImportCSV(enterpriseMemberImportMaxRows))
	require.NoError(t, err)
	require.Len(t, rows, enterpriseMemberImportMaxRows)
	require.Equal(t, "member-0001", rows[0].MemberCode)
	require.Equal(t, "member-5000", rows[len(rows)-1].MemberCode)

	_, err = parseEnterpriseMemberImportCSV(buildEnterpriseMemberImportCSV(enterpriseMemberImportMaxRows + 1))
	require.ErrorContains(t, err, "too many rows")
}

func BenchmarkParseEnterpriseMemberImportCSV5000Rows(b *testing.B) {
	data := buildEnterpriseMemberImportCSV(enterpriseMemberImportMaxRows)
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for range b.N {
		rows, err := parseEnterpriseMemberImportCSV(data)
		if err != nil || len(rows) != enterpriseMemberImportMaxRows {
			b.Fatalf("parse capacity fixture: rows=%d err=%v", len(rows), err)
		}
	}
}

func buildEnterpriseMemberImportCSV(rowCount int) []byte {
	var data strings.Builder
	data.Grow(rowCount * 72)
	_, _ = data.WriteString("member_code,member_name,monthly_limit_usd,opening_used_usd,key_name,api_key,key_quota_usd,groups\n")
	for row := 1; row <= rowCount; row++ {
		_, _ = fmt.Fprintf(&data, "member-%04d,Member %04d,100,0,,,,1\n", row, row)
	}
	return []byte(data.String())
}

func TestEnterpriseMemberImportPreviewStoresOnlyTokenHashAndEncryptedKey(t *testing.T) {
	repo := &importPreviewRepoCapture{}
	svc := NewEnterpriseMemberImportService(repo, importTestEncryptor{}, &APIKeyService{})
	data := []byte("member_code,member_name,monthly_limit_usd,opening_used_usd,key_name,api_key,key_quota_usd,groups\nmember-1,Alice,100,0,Primary,abcdefghijklmnop,0,1\n")
	preview, err := svc.Preview(context.Background(), 7, "csv", data)
	require.NoError(t, err)
	require.Equal(t, int64(9), preview.JobID)
	require.NotEmpty(t, preview.Token)
	require.Empty(t, preview.Rows[0].APIKeyCiphertext)
	require.NotNil(t, repo.job)
	require.Empty(t, repo.job.Preview.Token)
	require.Equal(t, "encrypted:abcdefghijklmnop", repo.job.Preview.Rows[0].APIKeyCiphertext)
	require.NotEqual(t, preview.Token, repo.job.TokenHash)
}

func TestNormalizeEnterpriseMemberImportSelectionIsCanonicalAndRejectsInvalidRows(t *testing.T) {
	rows := []EnterpriseMemberImportRow{
		{RowNumber: 3, Valid: true},
		{RowNumber: 1, Valid: true},
		{RowNumber: 2, Valid: false},
	}

	selected, err := normalizeEnterpriseMemberImportSelection(rows, []int{3, 1, 3})
	require.NoError(t, err)
	require.Equal(t, []int{1, 3}, selected)

	all, err := normalizeEnterpriseMemberImportSelection(rows, nil)
	require.NoError(t, err)
	require.Equal(t, []int{1, 3}, all)

	_, err = normalizeEnterpriseMemberImportSelection(rows, []int{2})
	require.ErrorIs(t, err, ErrEnterpriseMemberImportInvalid)
}

func TestEnterpriseMemberImportQueueCommitStoresCanonicalSelectionAndHashes(t *testing.T) {
	previewToken := "result-token-value"
	repo := &importQueueRepo{job: &EnterpriseMemberImportJob{
		ID: 21, EnterpriseUserID: 7, TokenHash: hashEnterpriseMemberImportToken(previewToken), Status: "previewed",
		ExpiresAt: time.Now().Add(time.Hour),
		Preview:   EnterpriseMemberImportPreview{Rows: []EnterpriseMemberImportRow{{RowNumber: 3, Valid: true}, {RowNumber: 1, Valid: true}}},
	}}
	svc := NewEnterpriseMemberImportService(repo, importTestEncryptor{}, &APIKeyService{})

	queued, err := svc.QueueCommit(context.Background(), 7, 21, previewToken, []int{3, 1, 3}, "commit-key")
	require.NoError(t, err)
	require.Equal(t, int64(21), queued.JobID)
	require.Equal(t, "queued", queued.Status)
	require.Equal(t, []int{1, 3}, repo.queuedRows)
	require.Equal(t, hashEnterpriseMemberImportToken(previewToken), repo.queuedTokenHash)
	require.Equal(t, HashIdempotencyKey("commit-key"), repo.queuedKeyHash)
}

func TestEnterpriseMemberImportXLSXTemplateRoundTrip(t *testing.T) {
	data, err := EnterpriseMemberImportXLSXTemplate()
	require.NoError(t, err)
	rows, err := parseEnterpriseMemberImportXLSX(data)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "employee-001", rows[0].MemberCode)
	require.Equal(t, "示例成员", rows[0].MemberName)
	require.Equal(t, []int64{1}, rows[0].GroupIDs)
	require.Equal(t, "主密钥", rows[0].KeyName)
}

func TestEnterpriseMemberImportXLSXAcceptsLegacyEnglishHeaders(t *testing.T) {
	template, err := EnterpriseMemberImportXLSXTemplate()
	require.NoError(t, err)

	legacy := rewriteImportXLSXForTest(t, template, func(_ string, content string) string {
		return strings.NewReplacer(
			"成员编号", "member_code",
			"成员名称", "member_name",
			"5小时限额", "rate_limit_5h",
			"1天限额", "rate_limit_1d",
			"7天限额", "rate_limit_7d",
			"自然月预算（USD）", "monthly_limit_usd",
			"初始已用额度（USD）", "opening_used_usd",
			"密钥名称", "key_name",
			"API密钥", "api_key",
			"密钥额度（USD）", "key_quota_usd",
			"分组ID", "group_id",
			"顺序", "sort_order",
		).Replace(content)
	}, "", "")

	rows, err := parseEnterpriseMemberImportXLSX(legacy)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "employee-001", rows[0].MemberCode)
	require.Equal(t, []int64{1}, rows[0].GroupIDs)
}

func TestEnterpriseMemberImportXLSXRejectsFormulaAndExternalContent(t *testing.T) {
	template, err := EnterpriseMemberImportXLSXTemplate()
	require.NoError(t, err)

	withFormula := rewriteImportXLSXForTest(t, template, func(name, content string) string {
		if name == "xl/worksheets/sheet1.xml" {
			return strings.Replace(content, "</is></c>", "</is><f>1+1</f><v>2</v></c>", 1)
		}
		return content
	}, "", "")
	_, err = parseEnterpriseMemberImportXLSX(withFormula)
	require.ErrorContains(t, err, "formulas are not allowed")

	withExternalLink := rewriteImportXLSXForTest(t, template, nil, "xl/externalLinks/externalLink1.xml", "<externalLink/>")
	_, err = parseEnterpriseMemberImportXLSX(withExternalLink)
	require.ErrorContains(t, err, "unsupported active or external content")
}

func rewriteImportXLSXForTest(t *testing.T, data []byte, transform func(name, content string) string, extraName, extraContent string) []byte {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	require.NoError(t, err)
	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	for _, file := range reader.File {
		source, err := file.Open()
		require.NoError(t, err)
		content, err := io.ReadAll(source)
		require.NoError(t, err)
		require.NoError(t, source.Close())
		value := string(content)
		if transform != nil {
			value = transform(file.Name, value)
		}
		target, err := writer.Create(file.Name)
		require.NoError(t, err)
		_, err = io.WriteString(target, value)
		require.NoError(t, err)
	}
	if extraName != "" {
		target, err := writer.Create(extraName)
		require.NoError(t, err)
		_, err = io.WriteString(target, extraContent)
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())
	return output.Bytes()
}
