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

func (r *importPreviewRepoCapture) ValidateReferences(_ context.Context, _ int64, _ []string, _ []string, groupIDs []int64) (*EnterpriseMemberImportReferenceState, error) {
	authorized := make(map[int64]bool, len(groupIDs))
	versions := make(map[string]int64, len(groupIDs))
	for _, groupID := range groupIDs {
		authorized[groupID] = true
		versions[fmt.Sprintf("group:%d", groupID)] = 1
	}
	return &EnterpriseMemberImportReferenceState{ExistingMemberCodes: map[string]bool{}, ExistingKeys: map[string]bool{}, AuthorizedGroupIDs: authorized, VersionFingerprint: versions}, nil
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
func (r *importPreviewRepoCapture) QueueCommit(context.Context, int64, int64, string, []int, []int64, bool, string) (*EnterpriseMemberImportJob, error) {
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
	queuedGroups    []int64
	queuedActivate  bool
	queuedTokenHash string
	queuedKeyHash   string
}

func (r *importQueueRepo) GetJobByToken(_ context.Context, ownerID, jobID int64, tokenHash string) (*EnterpriseMemberImportJob, error) {
	if r.job == nil || r.job.EnterpriseUserID != ownerID || r.job.ID != jobID || r.job.TokenHash != tokenHash {
		return nil, ErrEnterpriseMemberImportExpired
	}
	return r.job, nil
}

func (r *importQueueRepo) QueueCommit(_ context.Context, ownerID, jobID int64, tokenHash string, selectedRows []int, defaultGroupIDs []int64, activateMembers bool, idempotencyKeyHash string) (*EnterpriseMemberImportJob, error) {
	r.queuedRows = append([]int(nil), selectedRows...)
	r.queuedGroups = append([]int64(nil), defaultGroupIDs...)
	r.queuedActivate = activateMembers
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
	require.Contains(t, string(template), "成员编号,用户名称,API Key")

	rows, err := parseEnterpriseMemberImportCSV(template)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Empty(t, rows[0].MemberCode)
	require.Equal(t, "示例成员", rows[0].MemberName)
	require.Equal(t, "迁移密钥", rows[0].KeyName)
	require.Empty(t, rows[0].GroupIDs)
	require.Equal(t, int64(100000), rows[0].TotalTokens)
}

func TestEnterpriseMemberImportCSVTemplatePreviewsWithoutSystemGroupIDs(t *testing.T) {
	repo := &importPreviewRepoCapture{}
	svc := NewEnterpriseMemberImportService(repo, importTestEncryptor{}, &APIKeyService{})

	preview, err := svc.Preview(context.Background(), 7, "csv", EnterpriseMemberImportCSVTemplate())
	require.NoError(t, err)
	require.Equal(t, 1, preview.ValidRows)
	require.Zero(t, preview.InvalidRows)
	require.Empty(t, preview.Rows[0].GroupIDs)
	require.Contains(t, preview.Rows[0].Warnings, "member_code_generated")
}

func TestEnterpriseMemberImportPreviewNegotiatesPolicyVersion(t *testing.T) {
	repo := &importPreviewRepoCapture{}
	svc := NewEnterpriseMemberImportService(repo, importTestEncryptor{}, &APIKeyService{})

	preview, err := svc.PreviewWithPolicy(context.Background(), 7, "csv", EnterpriseMemberImportCSVTemplate(), EnterpriseMemberImportPolicyExplicitActivation)
	require.NoError(t, err)
	require.Equal(t, EnterpriseMemberImportPolicyExplicitActivation, preview.ImportPolicyVersion)
	require.Equal(t, EnterpriseMemberImportPolicyExplicitActivation, repo.job.ImportPolicyVersion)

	legacyPreview, err := svc.PreviewWithPolicy(context.Background(), 7, "csv", EnterpriseMemberImportCSVTemplate(), EnterpriseMemberImportPolicyLegacyAutoActivate)
	require.NoError(t, err)
	require.Equal(t, EnterpriseMemberImportPolicyLegacyAutoActivate, legacyPreview.ImportPolicyVersion)
	require.Equal(t, EnterpriseMemberImportPolicyLegacyAutoActivate, repo.job.ImportPolicyVersion)
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

func TestEnterpriseMemberImportPreviewAcceptsCustomerUsageExportWithoutSystemFields(t *testing.T) {
	repo := &importPreviewRepoCapture{}
	svc := NewEnterpriseMemberImportService(repo, importTestEncryptor{}, &APIKeyService{})
	data := []byte("用户名称,api_key,消费金额,月限制金额,总消耗token数,总输入token数,总输出token数,总缓存token数,总缓存token写入数,总缓存token读取数\n张三,abcdefghijklmnop,30,100,100000,50000,30000,20000,12000,8000\n")

	preview, err := svc.Preview(context.Background(), 7, "csv", data)
	require.NoError(t, err)
	require.Equal(t, 1, preview.ValidRows)
	require.Zero(t, preview.InvalidRows)
	require.Len(t, preview.Rows, 1)
	row := preview.Rows[0]
	require.Regexp(t, `^import-[a-f0-9]{16}$`, row.MemberCode)
	require.Equal(t, "张三", row.MemberName)
	require.Equal(t, "imported-key-2", row.KeyName)
	require.Equal(t, 30.0, row.OpeningUsedUSD)
	require.Equal(t, 100.0, row.MonthlyLimitUSD)
	require.Equal(t, int64(100000), row.TotalTokens)
	require.Equal(t, int64(50000), row.InputTokens)
	require.Equal(t, int64(30000), row.OutputTokens)
	require.Equal(t, int64(20000), row.CacheTokens)
	require.Empty(t, row.GroupIDs)
	require.Contains(t, row.Warnings, "member_code_generated")
	require.Contains(t, row.Warnings, "key_name_generated")
	require.NotContains(t, row.Warnings, "token_total_mismatch")
}

func TestEnterpriseMemberImportPreviewReportsSpecificAPIKeyValidationReasons(t *testing.T) {
	repo := &importPreviewRepoCapture{}
	svc := NewEnterpriseMemberImportService(repo, importTestEncryptor{}, &APIKeyService{})
	data := []byte("用户名称,api_key\n短密钥,sk-123456\n超长密钥," + strings.Repeat("a", 129) + "\n非法字符,abcdefghijklmnop!\n")

	preview, err := svc.Preview(context.Background(), 7, "csv", data)
	require.NoError(t, err)
	require.Zero(t, preview.ValidRows)
	require.Equal(t, 3, preview.InvalidRows)
	require.Equal(t, []string{"api_key_too_short_16_9"}, preview.Rows[0].Errors)
	require.Equal(t, []string{"api_key_too_long_128_129"}, preview.Rows[1].Errors)
	require.Equal(t, []string{"api_key_invalid_characters"}, preview.Rows[2].Errors)
}

func TestEnterpriseMemberImportPreviewRejectsAmbiguousSameNameRowsWithoutMemberCodes(t *testing.T) {
	repo := &importPreviewRepoCapture{}
	svc := NewEnterpriseMemberImportService(repo, importTestEncryptor{}, &APIKeyService{})
	data := []byte("用户名称,api_key,消费金额,月限制金额\n张三,abcdefghijklmnop,10,100\n张三,qrstuvwxyzabcdef,20,100\n")

	preview, err := svc.Preview(context.Background(), 7, "csv", data)
	require.NoError(t, err)
	require.Zero(t, preview.ValidRows)
	require.Equal(t, 2, preview.InvalidRows)
	require.Len(t, preview.Rows, 2)
	require.Equal(t, preview.Rows[0].MemberCode, preview.Rows[1].MemberCode)
	for _, row := range preview.Rows {
		require.Contains(t, row.Errors, "member_identity_ambiguous")
		require.False(t, row.Valid)
	}
}

func TestValidateEnterpriseMemberImportRowsWarnsOnlyWhenKnownTokenTotalDiffers(t *testing.T) {
	rows := []EnterpriseMemberImportRow{
		{MemberCode: "member-1", MemberName: "Alice", TotalTokens: 80, InputTokens: 50, OutputTokens: 30},
		{MemberCode: "member-2", MemberName: "Bob", TotalTokens: 100, InputTokens: 50, OutputTokens: 30, CacheTokens: 20},
		{MemberCode: "member-3", MemberName: "Carol", TotalTokens: 100},
		{MemberCode: "member-4", MemberName: "Dave", TotalTokens: 90, InputTokens: 50, OutputTokens: 30},
	}

	validateEnterpriseMemberImportRows(rows)
	require.NotContains(t, rows[0].Warnings, "token_total_mismatch")
	require.NotContains(t, rows[1].Warnings, "token_total_mismatch")
	require.NotContains(t, rows[2].Warnings, "token_total_mismatch")
	require.Contains(t, rows[3].Warnings, "token_total_mismatch")
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

	queued, err := svc.QueueCommit(context.Background(), 7, 21, previewToken, []int{3, 1, 3}, []int64{2, 1, 2}, true, "commit-key")
	require.NoError(t, err)
	require.Equal(t, int64(21), queued.JobID)
	require.Equal(t, "queued", queued.Status)
	require.Equal(t, []int{1, 3}, repo.queuedRows)
	require.Equal(t, []int64{2, 1}, repo.queuedGroups)
	require.True(t, repo.queuedActivate)
	require.Equal(t, hashEnterpriseMemberImportToken(previewToken), repo.queuedTokenHash)
	require.Equal(t, HashIdempotencyKey("commit-key"), repo.queuedKeyHash)
}

func TestEnterpriseMemberImportQueueCommitAllowsPendingMembersButRejectsActivationWithoutGroups(t *testing.T) {
	previewToken := "pending-import-token"
	repo := &importQueueRepo{job: &EnterpriseMemberImportJob{
		ID: 22, EnterpriseUserID: 7, TokenHash: hashEnterpriseMemberImportToken(previewToken), Status: "previewed",
		ExpiresAt: time.Now().Add(time.Hour),
		Preview:   EnterpriseMemberImportPreview{Rows: []EnterpriseMemberImportRow{{RowNumber: 1, Valid: true}}},
	}}
	svc := NewEnterpriseMemberImportService(repo, importTestEncryptor{}, &APIKeyService{})

	queued, err := svc.QueueCommit(context.Background(), 7, 22, previewToken, []int{1}, nil, false, "pending-commit")
	require.NoError(t, err)
	require.Equal(t, "queued", queued.Status)
	require.False(t, repo.queuedActivate)

	repo.job.ImportPolicyVersion = EnterpriseMemberImportPolicyExplicitActivation
	repo.job.Preview.Rows[0].GroupIDs = []int64{9}
	_, err = svc.QueueCommit(context.Background(), 7, 22, previewToken, []int{1}, nil, true, "activate-without-groups")
	require.ErrorIs(t, err, ErrEnterpriseMemberImportInvalid,
		"policy v2 must not treat historical row groups as the owner-selected system authorization")
}

func TestEnterpriseMemberImportPublicStatusHidesQueueProtocolVersion(t *testing.T) {
	require.Equal(t, "queued", enterpriseMemberImportPublicStatus(EnterpriseMemberImportStatusQueuedV2))
	require.Equal(t, "processing", enterpriseMemberImportPublicStatus(EnterpriseMemberImportStatusProcessingV2))
	require.Equal(t, "completed", enterpriseMemberImportPublicStatus("completed"))
	require.True(t, enterpriseMemberImportIsQueuedOrProcessing(EnterpriseMemberImportStatusQueuedV2))
	require.True(t, enterpriseMemberImportIsQueuedOrProcessing(EnterpriseMemberImportStatusProcessingV2))
}

func TestEnterpriseMemberImportXLSXTemplateRoundTrip(t *testing.T) {
	data, err := EnterpriseMemberImportXLSXTemplate()
	require.NoError(t, err)
	rows, err := parseEnterpriseMemberImportXLSX(data)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Empty(t, rows[0].MemberCode)
	require.Equal(t, "示例成员", rows[0].MemberName)
	require.Empty(t, rows[0].GroupIDs)
	require.Equal(t, "迁移密钥", rows[0].KeyName)
	require.Equal(t, 30.0, rows[0].OpeningUsedUSD)
	require.Equal(t, int64(100000), rows[0].TotalTokens)
}

func TestEnterpriseMemberImportXLSXAcceptsLegacyEnglishHeaders(t *testing.T) {
	template, err := EnterpriseMemberImportXLSXTemplate()
	require.NoError(t, err)

	legacy := rewriteImportXLSXForTest(t, template, func(_ string, content string) string {
		return strings.NewReplacer(
			"成员编号", "member_code",
			"用户名称", "member_name",
			"5小时限额", "rate_limit_5h",
			"1天限额", "rate_limit_1d",
			"7天限额", "rate_limit_7d",
			"月限制金额", "monthly_limit_usd",
			"本月已消费金额（USD）", "opening_used_usd",
			"密钥名称", "key_name",
			"API密钥", "api_key",
			"密钥额度（USD）", "key_quota_usd",
			"总消耗Token数", "total_tokens",
			"总输入Token数", "input_tokens",
			"总输出Token数", "output_tokens",
		).Replace(content)
	}, "", "")

	rows, err := parseEnterpriseMemberImportXLSX(legacy)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Empty(t, rows[0].MemberCode)
	require.Empty(t, rows[0].GroupIDs)
	require.Equal(t, int64(100000), rows[0].TotalTokens)
}

func TestEnterpriseMemberImportXLSXResolvesKeysByNameAndDoesNotDropUnknownMembers(t *testing.T) {
	rows, err := enterpriseMemberRowsFromXLSXSheets(map[string]importXLSXSheet{
		"members": {Rows: [][]string{
			{"成员编号", "用户名称", "月限制金额"},
			{"employee-001", "Alice", "100"},
		}},
		"keys": {Rows: [][]string{
			{"成员编号", "用户名称", "API Key", "总消耗Token数"},
			{"", "Alice", "abcdefghijklmnop", "80"},
			{"", "Missing", "qrstuvwxyzabcdef", "20"},
		}},
	})
	require.NoError(t, err)
	require.Len(t, rows, 2)
	require.Equal(t, "employee-001", rows[0].MemberCode)
	require.Equal(t, "abcdefghijklmnop", rows[0].APIKeyCiphertext)
	require.Equal(t, int64(80), rows[0].TotalTokens)
	require.Equal(t, "Missing", rows[1].MemberName)
	require.Contains(t, rows[1].Errors, "member_not_found_in_members_sheet")
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
