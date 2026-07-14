package handler

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestEnterpriseMemberImportTemplateContentDispositionUsesChineseFilename(t *testing.T) {
	require.Equal(
		t,
		`attachment; filename="enterprise-members-template.csv"; filename*=UTF-8''%E4%BC%81%E4%B8%9A%E6%88%90%E5%91%98%E5%AF%BC%E5%85%A5%E6%A8%A1%E6%9D%BF.csv`,
		enterpriseMemberImportTemplateContentDisposition("csv"),
	)
}

func TestParseEnterpriseMemberImportPolicyVersionDefaultsToExplicitActivation(t *testing.T) {
	version, ok := parseEnterpriseMemberImportPolicyVersion("")
	require.True(t, ok)
	require.Equal(t, service.EnterpriseMemberImportPolicyExplicitActivation, version)

	version, ok = parseEnterpriseMemberImportPolicyVersion(" 2 ")
	require.True(t, ok)
	require.Equal(t, service.EnterpriseMemberImportPolicyExplicitActivation, version)

	_, ok = parseEnterpriseMemberImportPolicyVersion("1")
	require.False(t, ok)

	_, ok = parseEnterpriseMemberImportPolicyVersion("invalid")
	require.False(t, ok)
}
