//go:build unit

package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// --- helpers ---

func testTimePtr(t time.Time) *time.Time { return &t }

func testFloatPtr(v float64) *float64 { return &v }

func makeAccWithLoad(id int64, priority int, loadRate int, lastUsed *time.Time, accType string) accountWithLoad {
	return accountWithLoad{
		account: &Account{
			ID:          id,
			Priority:    priority,
			LastUsedAt:  lastUsed,
			Type:        accType,
			Schedulable: true,
			Status:      StatusActive,
		},
		loadInfo: &AccountLoadInfo{
			AccountID:          id,
			CurrentConcurrency: 0,
			LoadRate:           loadRate,
		},
	}
}

// --- sortAccountsByPriorityAndLastUsed ---

func TestSortAccountsByPriorityAndLastUsed_ByPriority(t *testing.T) {
	now := time.Now()
	accounts := []*Account{
		{ID: 1, Priority: 5, LastUsedAt: testTimePtr(now)},
		{ID: 2, Priority: 1, LastUsedAt: testTimePtr(now)},
		{ID: 3, Priority: 3, LastUsedAt: testTimePtr(now)},
	}
	sortAccountsByPriorityAndLastUsed(accounts, false)
	require.Equal(t, int64(2), accounts[0].ID, "优先级最低的排第一")
	require.Equal(t, int64(3), accounts[1].ID)
	require.Equal(t, int64(1), accounts[2].ID)
}

func TestSortAccountsByPriorityAndLastUsed_SamePriorityByLastUsed(t *testing.T) {
	now := time.Now()
	accounts := []*Account{
		{ID: 1, Priority: 1, LastUsedAt: testTimePtr(now)},
		{ID: 2, Priority: 1, LastUsedAt: testTimePtr(now.Add(-1 * time.Hour))},
		{ID: 3, Priority: 1, LastUsedAt: nil},
	}
	sortAccountsByPriorityAndLastUsed(accounts, false)
	require.Equal(t, int64(3), accounts[0].ID, "nil LastUsedAt 排最前")
	require.Equal(t, int64(2), accounts[1].ID, "更早使用的排前面")
	require.Equal(t, int64(1), accounts[2].ID)
}

func TestSortAccountsByPriorityAndLastUsed_PreferOAuth(t *testing.T) {
	accounts := []*Account{
		{ID: 1, Priority: 1, LastUsedAt: nil, Type: AccountTypeAPIKey},
		{ID: 2, Priority: 1, LastUsedAt: nil, Type: AccountTypeOAuth},
	}
	sortAccountsByPriorityAndLastUsed(accounts, true)
	require.Equal(t, int64(2), accounts[0].ID, "preferOAuth 时 OAuth 账号排前面")
}

func TestSortAccountsByPriorityAndLastUsed_StableSort(t *testing.T) {
	accounts := []*Account{
		{ID: 1, Priority: 1, LastUsedAt: nil, Type: AccountTypeAPIKey},
		{ID: 2, Priority: 1, LastUsedAt: nil, Type: AccountTypeAPIKey},
		{ID: 3, Priority: 1, LastUsedAt: nil, Type: AccountTypeAPIKey},
	}

	// sortAccountsByPriorityAndLastUsed 内部会在同组(Priority+LastUsedAt)内做随机打散，
	// 因此这里不再断言“稳定排序”。我们只验证：
	// 1) 元素集合不变；2) 多次运行能产生不同的顺序。
	seenFirst := map[int64]bool{}
	for i := 0; i < 100; i++ {
		cpy := make([]*Account, len(accounts))
		copy(cpy, accounts)
		sortAccountsByPriorityAndLastUsed(cpy, false)
		seenFirst[cpy[0].ID] = true

		ids := map[int64]bool{}
		for _, a := range cpy {
			ids[a.ID] = true
		}
		require.True(t, ids[1] && ids[2] && ids[3])
	}
	require.GreaterOrEqual(t, len(seenFirst), 2, "同组账号应能被随机打散")
}

func TestSortAccountsByPriorityAndLastUsed_MixedPriorityAndTime(t *testing.T) {
	now := time.Now()
	accounts := []*Account{
		{ID: 1, Priority: 2, LastUsedAt: nil},
		{ID: 2, Priority: 1, LastUsedAt: testTimePtr(now)},
		{ID: 3, Priority: 1, LastUsedAt: testTimePtr(now.Add(-1 * time.Hour))},
		{ID: 4, Priority: 2, LastUsedAt: testTimePtr(now.Add(-2 * time.Hour))},
	}
	sortAccountsByPriorityAndLastUsed(accounts, false)
	// 优先级1排前：nil < earlier
	require.Equal(t, int64(3), accounts[0].ID, "优先级1 + 更早")
	require.Equal(t, int64(2), accounts[1].ID, "优先级1 + 现在")
	// 优先级2排后：nil < time
	require.Equal(t, int64(1), accounts[2].ID, "优先级2 + nil")
	require.Equal(t, int64(4), accounts[3].ID, "优先级2 + 有时间")
}

// --- filterByMinPriority ---

func TestFilterByMinPriority_Empty(t *testing.T) {
	result := filterByMinPriority(nil)
	require.Nil(t, result)
}

func TestFilterByMinPriority_SelectsMinPriority(t *testing.T) {
	accounts := []accountWithLoad{
		makeAccWithLoad(1, 5, 10, nil, AccountTypeAPIKey),
		makeAccWithLoad(2, 1, 10, nil, AccountTypeAPIKey),
		makeAccWithLoad(3, 1, 20, nil, AccountTypeAPIKey),
		makeAccWithLoad(4, 2, 10, nil, AccountTypeAPIKey),
	}
	result := filterByMinPriority(accounts)
	require.Len(t, result, 2)
	require.Equal(t, int64(2), result[0].account.ID)
	require.Equal(t, int64(3), result[1].account.ID)
}

func TestFilterByScheduleStrategy_StrictPriorityMatchesMinPriority(t *testing.T) {
	accounts := []accountWithLoad{
		makeAccWithLoad(1, 5, 10, nil, AccountTypeAPIKey),
		makeAccWithLoad(2, 1, 10, nil, AccountTypeAPIKey),
		makeAccWithLoad(3, 1, 20, nil, AccountTypeAPIKey),
	}
	accounts[0].account.UpstreamEffectiveDiscount = testFloatPtr(0.1)
	accounts[1].account.UpstreamEffectiveDiscount = testFloatPtr(0.9)
	accounts[2].account.UpstreamEffectiveDiscount = testFloatPtr(0.8)

	result := filterByScheduleStrategy(accounts, ScheduleStrategyStrictPriority)
	require.Equal(t, []int64{2, 3}, []int64{result[0].account.ID, result[1].account.ID})
}

func TestFilterByScheduleStrategy_CostFirstLowestDiscountThenPriority(t *testing.T) {
	accounts := []accountWithLoad{
		makeAccWithLoad(1, 5, 10, nil, AccountTypeAPIKey),
		makeAccWithLoad(2, 3, 10, nil, AccountTypeAPIKey),
		makeAccWithLoad(3, 1, 10, nil, AccountTypeAPIKey),
	}
	accounts[0].account.UpstreamEffectiveDiscount = testFloatPtr(0.7)
	accounts[1].account.UpstreamEffectiveDiscount = testFloatPtr(0.4)
	accounts[2].account.UpstreamEffectiveDiscount = testFloatPtr(0.4)

	result := filterByScheduleStrategy(accounts, ScheduleStrategyCostFirst)
	require.Len(t, result, 1)
	require.Equal(t, int64(3), result[0].account.ID, "同折扣时回退 priority 最小账号")
}

func TestFilterByScheduleStrategy_CostFirstNilDiscountsSink(t *testing.T) {
	accounts := []accountWithLoad{
		makeAccWithLoad(1, 1, 10, nil, AccountTypeAPIKey),
		makeAccWithLoad(2, 5, 10, nil, AccountTypeAPIKey),
	}
	accounts[1].account.UpstreamEffectiveDiscount = testFloatPtr(0.8)

	result := filterByScheduleStrategy(accounts, ScheduleStrategyCostFirst)
	require.Len(t, result, 1)
	require.Equal(t, int64(2), result[0].account.ID, "有有效折扣时 nil 折扣不参与最低折扣组")
}

func TestFilterByScheduleStrategy_CostFirstAllNilFallsBackToPriority(t *testing.T) {
	accounts := []accountWithLoad{
		makeAccWithLoad(1, 4, 10, nil, AccountTypeAPIKey),
		makeAccWithLoad(2, 1, 10, nil, AccountTypeAPIKey),
		makeAccWithLoad(3, 1, 20, nil, AccountTypeAPIKey),
	}

	result := filterByScheduleStrategy(accounts, ScheduleStrategyCostFirst)
	require.Equal(t, []int64{2, 3}, []int64{result[0].account.ID, result[1].account.ID})
}

func TestSortAccountWithLoadsForSelection_CostFirstPrecedesPriority(t *testing.T) {
	now := time.Now()
	accounts := []accountWithLoad{
		makeAccWithLoad(1, 1, 10, testTimePtr(now), AccountTypeAPIKey),
		makeAccWithLoad(2, 5, 20, testTimePtr(now.Add(-time.Hour)), AccountTypeAPIKey),
		makeAccWithLoad(3, 2, 5, nil, AccountTypeAPIKey),
	}
	accounts[0].account.UpstreamEffectiveDiscount = testFloatPtr(0.9)
	accounts[1].account.UpstreamEffectiveDiscount = testFloatPtr(0.4)
	accounts[2].account.UpstreamEffectiveDiscount = testFloatPtr(0.5)

	sortAccountWithLoadsForSelection(accounts, ScheduleStrategyCostFirst)

	require.Equal(t, []int64{2, 3, 1}, []int64{
		accounts[0].account.ID,
		accounts[1].account.ID,
		accounts[2].account.ID,
	})
}

func TestSortAccountWithLoadsForSelection_StrictPriorityIgnoresDiscount(t *testing.T) {
	accounts := []accountWithLoad{
		makeAccWithLoad(1, 1, 30, nil, AccountTypeAPIKey),
		makeAccWithLoad(2, 2, 10, nil, AccountTypeAPIKey),
	}
	accounts[0].account.UpstreamEffectiveDiscount = testFloatPtr(0.9)
	accounts[1].account.UpstreamEffectiveDiscount = testFloatPtr(0.1)

	sortAccountWithLoadsForSelection(accounts, ScheduleStrategyStrictPriority)

	require.Equal(t, []int64{1, 2}, []int64{
		accounts[0].account.ID,
		accounts[1].account.ID,
	})
}

func TestSortAccountsByScheduleStrategyAndLastUsed_CostFirstNilDiscountLast(t *testing.T) {
	now := time.Now()
	accounts := []*Account{
		{ID: 1, Priority: 1, LastUsedAt: testTimePtr(now), UpstreamEffectiveDiscount: nil},
		{ID: 2, Priority: 5, LastUsedAt: testTimePtr(now.Add(-time.Hour)), UpstreamEffectiveDiscount: testFloatPtr(0.6)},
		{ID: 3, Priority: 3, LastUsedAt: nil, UpstreamEffectiveDiscount: testFloatPtr(0.4)},
	}

	sortAccountsByScheduleStrategyAndLastUsed(accounts, ScheduleStrategyCostFirst, false)

	require.Equal(t, []int64{3, 2, 1}, []int64{accounts[0].ID, accounts[1].ID, accounts[2].ID})
}

func TestIsBetterAccountByScheduleStrategy_PreferOAuthSamePlatformOnlyGemini(t *testing.T) {
	anthropicCurrent := &Account{ID: 1, Platform: PlatformAnthropic, Priority: 1, Type: AccountTypeAPIKey}
	anthropicOAuth := &Account{ID: 2, Platform: PlatformAnthropic, Priority: 1, Type: AccountTypeOAuth}
	require.False(t,
		isBetterAccountByScheduleStrategy(anthropicOAuth, anthropicCurrent, ScheduleStrategyStrictPriority, true, true),
		"mixed scheduling 旧逻辑只允许 Gemini-vs-Gemini 的 OAuth tie-break")

	geminiCurrent := &Account{ID: 3, Platform: PlatformGemini, Priority: 1, Type: AccountTypeAPIKey}
	geminiOAuth := &Account{ID: 4, Platform: PlatformGemini, Priority: 1, Type: AccountTypeOAuth}
	require.True(t,
		isBetterAccountByScheduleStrategy(geminiOAuth, geminiCurrent, ScheduleStrategyStrictPriority, true, true),
		"Gemini-vs-Gemini 仍应保留 OAuth tie-break")
}

// --- filterByMinLoadRate ---

func TestFilterByMinLoadRate_Empty(t *testing.T) {
	result := filterByMinLoadRate(nil)
	require.Nil(t, result)
}

func TestFilterByMinLoadRate_SelectsMinLoadRate(t *testing.T) {
	accounts := []accountWithLoad{
		makeAccWithLoad(1, 1, 30, nil, AccountTypeAPIKey),
		makeAccWithLoad(2, 1, 10, nil, AccountTypeAPIKey),
		makeAccWithLoad(3, 1, 10, nil, AccountTypeAPIKey),
		makeAccWithLoad(4, 1, 20, nil, AccountTypeAPIKey),
	}
	result := filterByMinLoadRate(accounts)
	require.Len(t, result, 2)
	require.Equal(t, int64(2), result[0].account.ID)
	require.Equal(t, int64(3), result[1].account.ID)
}

// --- selectByLRU ---

func TestSelectByLRU_Empty(t *testing.T) {
	result := selectByLRU(nil, false)
	require.Nil(t, result)
}

func TestSelectByLRU_Single(t *testing.T) {
	accounts := []accountWithLoad{makeAccWithLoad(1, 1, 10, nil, AccountTypeAPIKey)}
	result := selectByLRU(accounts, false)
	require.NotNil(t, result)
	require.Equal(t, int64(1), result.account.ID)
}

func TestSelectByLRU_NilLastUsedAtWins(t *testing.T) {
	now := time.Now()
	accounts := []accountWithLoad{
		makeAccWithLoad(1, 1, 10, testTimePtr(now), AccountTypeAPIKey),
		makeAccWithLoad(2, 1, 10, nil, AccountTypeAPIKey),
		makeAccWithLoad(3, 1, 10, testTimePtr(now.Add(-1*time.Hour)), AccountTypeAPIKey),
	}
	result := selectByLRU(accounts, false)
	require.NotNil(t, result)
	require.Equal(t, int64(2), result.account.ID)
}

func TestSelectByLRU_EarliestTimeWins(t *testing.T) {
	now := time.Now()
	accounts := []accountWithLoad{
		makeAccWithLoad(1, 1, 10, testTimePtr(now), AccountTypeAPIKey),
		makeAccWithLoad(2, 1, 10, testTimePtr(now.Add(-1*time.Hour)), AccountTypeAPIKey),
		makeAccWithLoad(3, 1, 10, testTimePtr(now.Add(-2*time.Hour)), AccountTypeAPIKey),
	}
	result := selectByLRU(accounts, false)
	require.NotNil(t, result)
	require.Equal(t, int64(3), result.account.ID)
}

func TestSelectByLRU_TiePreferOAuth(t *testing.T) {
	now := time.Now()
	// 账号 1/2 LastUsedAt 相同，且同为最小值。
	accounts := []accountWithLoad{
		makeAccWithLoad(1, 1, 10, testTimePtr(now), AccountTypeAPIKey),
		makeAccWithLoad(2, 1, 10, testTimePtr(now), AccountTypeOAuth),
		makeAccWithLoad(3, 1, 10, testTimePtr(now.Add(1*time.Hour)), AccountTypeAPIKey),
	}
	for i := 0; i < 50; i++ {
		result := selectByLRU(accounts, true)
		require.NotNil(t, result)
		require.Equal(t, AccountTypeOAuth, result.account.Type)
		require.Equal(t, int64(2), result.account.ID)
	}
}
