package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

type supplierBalanceTestEncryptor struct{}

func (supplierBalanceTestEncryptor) Encrypt(value string) (string, error) {
	return "encrypted:" + value, nil
}
func (supplierBalanceTestEncryptor) Decrypt(value string) (string, error) {
	return "", errors.New("test decrypt failure")
}

func TestSupplierBalanceConfigOptionalAndPreservesToken(t *testing.T) {
	current := &UpstreamSupplier{ID: 1}
	cfg, token, _, err := normalizeSupplierBalanceInput(current, UpstreamSupplierBalanceInput{}, nil)
	require.NoError(t, err)
	require.False(t, cfg.Enabled)
	require.Empty(t, token)

	input := UpstreamSupplierBalanceInput{Enabled: true, Provider: "newapi", BaseURL: "https://supplier.example/", UserID: 7, AccessToken: " Bearer secret "}
	cfg, token, reset, err := normalizeSupplierBalanceInput(current, input, supplierBalanceTestEncryptor{})
	require.NoError(t, err)
	require.Equal(t, "encrypted:secret", token)
	require.True(t, reset)
	require.True(t, cfg.HasAccessToken)
	require.Equal(t, "https://supplier.example", cfg.BaseURL)
	current.BalanceConfig, current.balanceAccessToken = cfg, token
	for _, blank := range []string{"", "   "} {
		input.AccessToken = blank
		cfg, kept, reset, err := normalizeSupplierBalanceInput(current, input, nil)
		require.NoError(t, err)
		require.Equal(t, token, kept)
		require.True(t, cfg.HasAccessToken)
		require.False(t, reset)
	}
	input.Enabled = false
	_, kept, _, err := normalizeSupplierBalanceInput(current, input, nil)
	require.NoError(t, err)
	require.Equal(t, token, kept)
}

func TestSupplierBalanceConfigRequiresCredentialsForNewSite(t *testing.T) {
	current := &UpstreamSupplier{BalanceConfig: UpstreamSupplierBalanceConfig{Enabled: true, Provider: "newapi", BaseURL: "https://first.example"}, balanceAccessToken: "saved"}
	_, _, _, err := normalizeSupplierBalanceInput(current, UpstreamSupplierBalanceInput{Enabled: true, Provider: "newapi", BaseURL: "https://second.example"}, nil)
	require.ErrorContains(t, err, "enter a new token")
	for _, website := range []string{"file:///etc/passwd", "https://user:pass@site.example", "https://site.example?access_token=secret", "https://site.example/#secret"} {
		_, _, _, err = normalizeSupplierBalanceInput(current, UpstreamSupplierBalanceInput{BaseURL: website}, nil)
		require.Error(t, err)
	}
	_, _, _, err = normalizeSupplierBalanceInput(current, UpstreamSupplierBalanceInput{Provider: "unknown"}, nil)
	require.Error(t, err)
}

func TestSupplierBalanceAPIKeyConfigPreservesSameProviderCredential(t *testing.T) {
	for _, provider := range []string{"soleapi", "sub2api"} {
		t.Run(provider, func(t *testing.T) {
			input := UpstreamSupplierBalanceInput{Enabled: true, Provider: provider, BaseURL: "https://supplier.example", UserID: 42, AccessToken: "sk-supplier-test"}
			cfg, ciphertext, reset, err := normalizeSupplierBalanceInput(&UpstreamSupplier{}, input, supplierBalanceTestEncryptor{})
			require.NoError(t, err)
			require.Equal(t, provider, cfg.Provider)
			require.Zero(t, cfg.UserID)
			require.Equal(t, "encrypted:sk-supplier-test", ciphertext)
			require.True(t, reset)
			current := &UpstreamSupplier{BalanceConfig: cfg, balanceAccessToken: ciphertext}
			input.AccessToken = ""
			_, kept, reset, err := normalizeSupplierBalanceInput(current, input, nil)
			require.NoError(t, err)
			require.Equal(t, ciphertext, kept)
			require.False(t, reset)
		})
	}
}

func TestSupplierBalanceProviderChangeRequiresNewCredential(t *testing.T) {
	for _, from := range []string{"newapi", "soleapi", "sub2api"} {
		for _, to := range []string{"newapi", "soleapi", "sub2api"} {
			if from == to {
				continue
			}
			t.Run(from+" to "+to, func(t *testing.T) {
				current := &UpstreamSupplier{BalanceConfig: UpstreamSupplierBalanceConfig{Enabled: true, Provider: from, BaseURL: "https://supplier.example"}, balanceAccessToken: "old-ciphertext"}
				input := UpstreamSupplierBalanceInput{Enabled: true, Provider: to, BaseURL: current.BalanceConfig.BaseURL}
				_, _, _, err := normalizeSupplierBalanceInput(current, input, nil)
				require.ErrorContains(t, err, "enter a new token")
				input.AccessToken = "replacement"
				_, ciphertext, reset, err := normalizeSupplierBalanceInput(current, input, supplierBalanceTestEncryptor{})
				require.NoError(t, err)
				require.Equal(t, "encrypted:replacement", ciphertext)
				require.True(t, reset)
			})
		}
	}
}

func TestSupplierBalanceSnapshotRetainsLastSuccessAndRealZero(t *testing.T) {
	first := time.Date(2026, 9, 10, 1, 0, 0, 0, time.UTC)
	second := first.Add(time.Minute)
	previous := supplierBalanceSnapshot(nil, 12.5, nil, first)
	failed := supplierBalanceSnapshot(previous, 0, errors.New("upstream HTTP 401"), second)
	require.Equal(t, 12.5, *failed.BalanceUSD)
	require.Equal(t, first, *failed.UpdatedAt)
	require.Equal(t, second, failed.LastAttemptAt)
	require.Equal(t, "error", failed.Status)
	require.Equal(t, "ok", previous.Status)
	unknown := supplierBalanceSnapshot(nil, 0, errors.New("timeout"), second)
	require.Nil(t, unknown.BalanceUSD)
	require.Nil(t, unknown.UpdatedAt)
	zero := supplierBalanceSnapshot(failed, 0, nil, second)
	require.NotNil(t, zero.BalanceUSD)
	require.Zero(t, *zero.BalanceUSD)
	require.Empty(t, zero.Error)
}

func TestSupplierBalanceSerializationNeverExposesCredential(t *testing.T) {
	supplier := UpstreamSupplier{BalanceConfig: UpstreamSupplierBalanceConfig{HasAccessToken: true}, balanceAccessToken: "private-ciphertext"}
	encoded, err := json.Marshal(supplier)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "private-ciphertext")
	require.NotContains(t, string(encoded), `"access_token"`)
	require.Contains(t, string(encoded), `"has_access_token":true`)
}

func TestSupplierBalanceConfigurationStoresOnlyEncryptedToken(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	s := &adminServiceImpl{secretEncryptor: supplierBalanceTestEncryptor{}}
	current := &UpstreamSupplier{ID: 1}
	mock.ExpectExec("UPDATE upstream_suppliers").WithArgs(int64(1), sqlmock.AnyArg(), "encrypted:secret", true).WillReturnResult(sqlmock.NewResult(0, 1))
	err = s.configureUpstreamSupplierBalance(context.Background(), db, current, &UpstreamSupplierBalanceInput{Enabled: true, Provider: "newapi", BaseURL: "https://supplier.example", AccessToken: "secret"})
	require.NoError(t, err)
	require.NoError(t, s.configureUpstreamSupplierBalance(context.Background(), db, current, nil))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSupplierBalanceConfigurationErrorContext(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	s := &adminServiceImpl{secretEncryptor: supplierBalanceTestEncryptor{}}
	current := &UpstreamSupplier{ID: 1}
	input := &UpstreamSupplierBalanceInput{Enabled: true, Provider: "newapi", BaseURL: "https://supplier.example", AccessToken: "synthetic-token"}
	dbErr := errors.New("database unavailable")
	mock.ExpectExec("UPDATE upstream_suppliers").WillReturnError(dbErr)
	err = s.configureUpstreamSupplierBalance(context.Background(), db, current, input)
	require.ErrorIs(t, err, dbErr)
	require.ErrorContains(t, err, "save supplier balance configuration")
	require.NotContains(t, err.Error(), "synthetic-token")

	input.Provider = "unsupported"
	err = s.configureUpstreamSupplierBalance(context.Background(), db, current, input)
	require.Equal(t, "INVALID_SUPPLIER_BALANCE_CONFIG", infraerrors.Reason(err))
	require.NoError(t, mock.ExpectationsWereMet())
}
