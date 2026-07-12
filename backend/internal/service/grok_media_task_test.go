package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSelectGrokMediaTaskAccountUsesPersistedAccountOnly(t *testing.T) {
	repo := stubOpenAIAccountRepo{accounts: []Account{
		{ID: 63, Platform: PlatformGrok, Status: StatusActive, Concurrency: 1, GroupIDs: []int64{12}},
		{ID: 64, Platform: PlatformGrok, Status: StatusActive, Concurrency: 1, GroupIDs: []int64{12}},
	}}
	svc := &OpenAIGatewayService{accountRepo: repo}

	selection, err := svc.SelectGrokMediaTaskAccount(context.Background(), 63, 12)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.Equal(t, int64(63), selection.Account.ID)
	require.True(t, selection.Acquired)

	_, err = svc.SelectGrokMediaTaskAccount(context.Background(), 63, 99)
	require.ErrorIs(t, err, ErrGrokMediaTaskNotFound)
}
