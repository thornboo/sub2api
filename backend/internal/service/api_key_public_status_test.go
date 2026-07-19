package service

import (
	"testing"
	"time"
)

func TestBuildAPIKeyPublicStatusReflectsStaticAccessEligibility(t *testing.T) {
	memberID := int64(22)
	groupID := int64(33)
	deletedAt := time.Now()
	activeUser := &User{ID: 7, Status: StatusActive, Role: RoleUser}
	enterpriseUser := &User{ID: 7, Status: StatusActive, Role: RoleUser, AccountType: UserAccountTypeEnterprise}
	activeGroup := Group{ID: groupID, Status: StatusActive, Hydrated: true, Platform: PlatformOpenAI}

	tests := []struct {
		name   string
		apiKey *APIKey
		active bool
	}{
		{
			name:   "ordinary active key",
			apiKey: &APIKey{Status: StatusAPIKeyActive, UserID: activeUser.ID, User: activeUser},
			active: true,
		},
		{
			name:   "inactive owner",
			apiKey: &APIKey{Status: StatusAPIKeyActive, UserID: 7, User: &User{ID: 7, Status: StatusDisabled}},
		},
		{
			name: "disabled fixed group",
			apiKey: &APIKey{Status: StatusAPIKeyActive, UserID: activeUser.ID, User: activeUser, GroupID: &groupID,
				Group: &Group{ID: groupID, Status: StatusDisabled, Hydrated: true, Platform: PlatformOpenAI}},
		},
		{
			name: "disabled enterprise member",
			apiKey: &APIKey{Status: StatusAPIKeyActive, UserID: enterpriseUser.ID, User: enterpriseUser, MemberID: &memberID,
				Member: &EnterpriseMember{ID: memberID, EnterpriseUserID: enterpriseUser.ID, Status: EnterpriseMemberStatusDisabled, Groups: []Group{activeGroup}}},
		},
		{
			name: "archived enterprise member",
			apiKey: &APIKey{Status: StatusAPIKeyActive, UserID: enterpriseUser.ID, User: enterpriseUser, MemberID: &memberID,
				Member: &EnterpriseMember{ID: memberID, EnterpriseUserID: enterpriseUser.ID, Status: EnterpriseMemberStatusActive, DeletedAt: &deletedAt, Groups: []Group{activeGroup}}},
		},
		{
			name: "enterprise member without active group",
			apiKey: &APIKey{Status: StatusAPIKeyActive, UserID: enterpriseUser.ID, User: enterpriseUser, MemberID: &memberID,
				Member: &EnterpriseMember{ID: memberID, EnterpriseUserID: enterpriseUser.ID, Status: EnterpriseMemberStatusActive, Groups: []Group{{ID: groupID, Status: StatusDisabled, Hydrated: true, Platform: PlatformOpenAI}}}},
		},
		{
			name: "enterprise member with revoked exclusive group",
			apiKey: &APIKey{Status: StatusAPIKeyActive, UserID: enterpriseUser.ID, User: enterpriseUser, MemberID: &memberID,
				Member: &EnterpriseMember{ID: memberID, EnterpriseUserID: enterpriseUser.ID, Status: EnterpriseMemberStatusActive, Groups: []Group{{ID: groupID, Status: StatusActive, Hydrated: true, Platform: PlatformOpenAI, IsExclusive: true}}}},
		},
		{
			name: "active enterprise member",
			apiKey: &APIKey{Status: StatusAPIKeyActive, UserID: enterpriseUser.ID, User: enterpriseUser, MemberID: &memberID,
				Member: &EnterpriseMember{ID: memberID, EnterpriseUserID: enterpriseUser.ID, Status: EnterpriseMemberStatusActive, Groups: []Group{activeGroup}}},
			active: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			status := buildAPIKeyPublicStatus(test.apiKey)
			if status.IsActive != test.active {
				t.Fatalf("IsActive = %v, want %v", status.IsActive, test.active)
			}
			if !test.active && status.Status != StatusAPIKeyDisabled {
				t.Fatalf("Status = %q, want %q", status.Status, StatusAPIKeyDisabled)
			}
		})
	}
}
