package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRoutingEligibilityRevisionIgnoresDuplicateAndOlderEvents(t *testing.T) {
	mirror := NewRoutingEligibilityRevisionMirror()
	scope := RoutingEligibilityScope{Type: RoutingEligibilityScopeGroup, ID: 42}

	require.True(t, mirror.Apply(scope, 10))
	require.False(t, mirror.Apply(scope, 10))
	require.False(t, mirror.Apply(scope, 9))
	require.EqualValues(t, 10, mirror.Revision(scope))

	require.True(t, mirror.Apply(RoutingEligibilityScope{Type: " GROUP ", ID: 42}, 11))
	require.EqualValues(t, 11, mirror.Revision(scope))
}

func TestRoutingEligibilityRevisionUsesExplicitScopeTypes(t *testing.T) {
	mirror := NewRoutingEligibilityRevisionMirror()
	scopes := []RoutingEligibilityScope{
		{Type: RoutingEligibilityScopeChannel, ID: 1},
		{Type: RoutingEligibilityScopeGroup, ID: 2},
		{Type: RoutingEligibilityScopeAccount, ID: 3},
		{Type: RoutingEligibilityScopeProtocol, ID: 4},
		{Type: RoutingEligibilityScopeComposite, ID: 5},
	}

	for i, scope := range scopes {
		require.True(t, mirror.Apply(scope, uint64(i+1)))
		require.EqualValues(t, i+1, mirror.Revision(scope))
	}
	require.False(t, mirror.Apply(RoutingEligibilityScope{Type: "unknown", ID: 1}, 99))
	require.False(t, mirror.Apply(RoutingEligibilityScope{Type: RoutingEligibilityScopeGroup, ID: -1}, 99))
}

func TestRoutingEligibilityRevisionVersionIsDeterministicForScopeSet(t *testing.T) {
	mirror := NewRoutingEligibilityRevisionMirror()
	require.True(t, mirror.Apply(RoutingEligibilityScope{Type: RoutingEligibilityScopeAccount, ID: 7}, 70))
	require.True(t, mirror.Apply(RoutingEligibilityScope{Type: RoutingEligibilityScopeGroup, ID: 3}, 30))
	require.True(t, mirror.Apply(RoutingEligibilityScope{Type: RoutingEligibilityScopeChannel, ID: 5}, 50))

	first := mirror.VersionFor([]RoutingEligibilityScope{
		{Type: RoutingEligibilityScopeAccount, ID: 7},
		{Type: RoutingEligibilityScopeGroup, ID: 3},
		{Type: RoutingEligibilityScopeChannel, ID: 5},
		{Type: RoutingEligibilityScopeGroup, ID: 3},
	})
	second := mirror.VersionFor([]RoutingEligibilityScope{
		{Type: RoutingEligibilityScopeGroup, ID: 3},
		{Type: RoutingEligibilityScopeChannel, ID: 5},
		{Type: RoutingEligibilityScopeAccount, ID: 7},
	})

	require.True(t, first.Equal(second))
	require.Equal(t, first.Digest, second.Digest)
	require.Equal(t, []RoutingEligibilityScopeRevision{
		{Scope: RoutingEligibilityScope{Type: RoutingEligibilityScopeAccount, ID: 7}, Revision: 70},
		{Scope: RoutingEligibilityScope{Type: RoutingEligibilityScopeChannel, ID: 5}, Revision: 50},
		{Scope: RoutingEligibilityScope{Type: RoutingEligibilityScopeGroup, ID: 3}, Revision: 30},
	}, first.Items)
}

func TestRoutingEligibilityRevisionVersionChangesWhenScopeRevisionChanges(t *testing.T) {
	mirror := NewRoutingEligibilityRevisionMirror()
	scope := RoutingEligibilityScope{Type: RoutingEligibilityScopeProtocol, ID: 9}
	require.True(t, mirror.Apply(scope, 1))
	before := mirror.VersionFor([]RoutingEligibilityScope{scope})

	require.True(t, mirror.Apply(scope, 2))
	after := mirror.VersionFor([]RoutingEligibilityScope{scope})

	require.NotEqual(t, before.Digest, after.Digest)
	require.EqualValues(t, 2, after.Items[0].Revision)
}

func TestRoutingEligibilityRevisionNextLocalRevisionIsMonotonicMirrorOnly(t *testing.T) {
	mirror := NewRoutingEligibilityRevisionMirror()
	groupScope := RoutingEligibilityScope{Type: RoutingEligibilityScopeGroup, ID: 1}
	accountScope := RoutingEligibilityScope{Type: RoutingEligibilityScopeAccount, ID: 2}

	require.Zero(t, mirror.NextLocalRevision(RoutingEligibilityScope{Type: "unknown", ID: 1}))
	first := mirror.NextLocalRevision(groupScope)
	second := mirror.NextLocalRevision(accountScope)

	require.Greater(t, second, first)
	require.Equal(t, first, mirror.Revision(groupScope))
	require.Equal(t, second, mirror.Revision(accountScope))
}
