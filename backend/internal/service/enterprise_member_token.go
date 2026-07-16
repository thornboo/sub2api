package service

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/shopspring/decimal"
)

var enterpriseMemberMaxTokenCount = decimal.RequireFromString("9223372036854775807.99")

const enterpriseMemberAggregateTokenLiteralMaxBytes = 128

// EnterpriseMemberTokenCount is exact external migration evidence. Unlike
// request usage counters, imported values and their aggregates may contain two
// decimal places. Aggregate values may exceed one persisted baseline column.
type EnterpriseMemberTokenCount struct {
	decimal decimal.Decimal
}

// ParseEnterpriseMemberTokenCount parses one canonical value that is safe to
// persist in a NUMERIC(21,2) baseline column.
func ParseEnterpriseMemberTokenCount(value string) (EnterpriseMemberTokenCount, error) {
	return parseEnterpriseMemberTokenCount(value, true)
}

func parseEnterpriseMemberAggregateTokenCount(value string) (EnterpriseMemberTokenCount, error) {
	return parseEnterpriseMemberTokenCount(value, false)
}

func parseEnterpriseMemberTokenCount(value string, enforcePersistedMax bool) (EnterpriseMemberTokenCount, error) {
	value = strings.TrimSpace(value)
	if !validEnterpriseMemberTokenLiteral(value, enterpriseMemberAggregateTokenLiteralMaxBytes) {
		return EnterpriseMemberTokenCount{}, errors.New("invalid token count")
	}
	parsed, err := decimal.NewFromString(value)
	if err != nil || !validEnterpriseMemberTokenCount(parsed) || (enforcePersistedMax && parsed.GreaterThan(enterpriseMemberMaxTokenCount)) {
		return EnterpriseMemberTokenCount{}, errors.New("invalid token count")
	}
	return EnterpriseMemberTokenCount{decimal: parsed}, nil
}

func validEnterpriseMemberTokenLiteral(value string, maxBytes int) bool {
	if value == "" || len(value) > maxBytes {
		return false
	}
	decimalPoint := -1
	for i := range value {
		if value[i] == '.' {
			if decimalPoint >= 0 || i == 0 || i == len(value)-1 {
				return false
			}
			decimalPoint = i
			continue
		}
		if value[i] < '0' || value[i] > '9' {
			return false
		}
	}
	return true
}

func validEnterpriseMemberTokenCount(value decimal.Decimal) bool {
	return !value.IsNegative() && value.Equal(value.Round(2))
}

func (count EnterpriseMemberTokenCount) Add(other EnterpriseMemberTokenCount) EnterpriseMemberTokenCount {
	return EnterpriseMemberTokenCount{decimal: count.decimal.Add(other.decimal)}
}

func (count EnterpriseMemberTokenCount) Equal(other EnterpriseMemberTokenCount) bool {
	return count.decimal.Equal(other.decimal)
}

func (count EnterpriseMemberTokenCount) IsZero() bool {
	return count.decimal.IsZero()
}

func (count EnterpriseMemberTokenCount) IsPositive() bool {
	return count.decimal.IsPositive()
}

func (count EnterpriseMemberTokenCount) IsPersistable() bool {
	return validEnterpriseMemberTokenCount(count.decimal) && !count.decimal.GreaterThan(enterpriseMemberMaxTokenCount)
}

func (count EnterpriseMemberTokenCount) String() string {
	return count.decimal.StringFixed(2)
}

func (count EnterpriseMemberTokenCount) MarshalJSON() ([]byte, error) {
	return json.Marshal(count.String())
}

func (count *EnterpriseMemberTokenCount) UnmarshalJSON(data []byte) error {
	value := strings.TrimSpace(string(data))
	if strings.HasPrefix(value, `"`) {
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
	}
	parsed, err := parseEnterpriseMemberAggregateTokenCount(value)
	if err != nil {
		return err
	}
	*count = parsed
	return nil
}

func (count *EnterpriseMemberTokenCount) Scan(value any) error {
	var parsed decimal.Decimal
	if err := parsed.Scan(value); err != nil {
		return err
	}
	if !validEnterpriseMemberTokenCount(parsed) {
		return errors.New("invalid token count")
	}
	count.decimal = parsed
	return nil
}

func (count EnterpriseMemberTokenCount) Value() (driver.Value, error) {
	if !count.IsPersistable() {
		return nil, fmt.Errorf("token count %s exceeds the persisted field limit", count.String())
	}
	return count.String(), nil
}
