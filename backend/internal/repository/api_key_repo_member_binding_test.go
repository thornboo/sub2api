package repository

import (
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
)

func TestAPIKeyEntityToServiceUsesMemberBindingSortOrder(t *testing.T) {
	entity := &dbent.APIKey{
		ID:     9,
		UserID: 7,
		Edges: dbent.APIKeyEdges{
			Member: &dbent.EnterpriseMember{
				ID: 22,
				Edges: dbent.EnterpriseMemberEdges{
					EnterpriseMemberGroupBindings: []*dbent.EnterpriseMemberGroupBinding{
						{
							GroupID:   33,
							SortOrder: 4,
							Edges: dbent.EnterpriseMemberGroupBindingEdges{
								Group: &dbent.Group{ID: 33, SortOrder: 99},
							},
						},
					},
				},
			},
		},
	}

	mapped := apiKeyEntityToService(entity)
	if mapped.Member == nil || len(mapped.Member.Groups) != 1 {
		t.Fatalf("member groups = %+v, want one group", mapped.Member)
	}
	if got := mapped.Member.Groups[0].SortOrder; got != 4 {
		t.Fatalf("member group sort order = %d, want binding order 4", got)
	}
}
