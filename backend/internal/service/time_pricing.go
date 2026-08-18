package service

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
)

const (
	maxTimePricingRules      = 16
	maxTimePricingLabelRunes = 32
)

var strictHHMMPattern = regexp.MustCompile(`^\d{2}:\d{2}$`)

type TimePricing struct {
	Enabled           bool              `json:"enabled"`
	Timezone          string            `json:"timezone"`
	DefaultLabel      string            `json:"default_label"`
	DefaultMultiplier *float64          `json:"default_multiplier,omitempty"`
	Rules             []TimePricingRule `json:"rules"`
}

type TimePricingRule struct {
	Label      string  `json:"label,omitempty"`
	StartTime  string  `json:"start_time"`
	EndTime    string  `json:"end_time"`
	Multiplier float64 `json:"multiplier"`
}

type AppliedTimePricing struct {
	Multiplier float64
	Timezone   string
	Label      string
	Rule       *TimePricingRule
}

func (p TimePricing) Clone() TimePricing {
	cp := p
	if p.DefaultMultiplier != nil {
		defaultMultiplier := *p.DefaultMultiplier
		cp.DefaultMultiplier = &defaultMultiplier
	}
	if p.Rules != nil {
		cp.Rules = make([]TimePricingRule, len(p.Rules))
		copy(cp.Rules, p.Rules)
	}
	return cp
}

func (p *TimePricing) IsActive() bool {
	if p == nil || !p.Enabled {
		return false
	}
	timezoneName := strings.TrimSpace(p.Timezone)
	if timezoneName == "" {
		return false
	}
	_, err := time.LoadLocation(timezoneName)
	return err == nil
}

func ValidateTimePricingForMode(p *TimePricing, mode BillingMode) error {
	if p == nil || !p.Enabled {
		return nil
	}
	if mode == "" {
		mode = BillingModeToken
	}
	if mode != BillingModeToken {
		return infraerrors.BadRequest("TIME_PRICING_TOKEN_ONLY", "time_pricing is only supported for token billing mode")
	}
	if _, err := validateTimePricing(p); err != nil {
		return err
	}
	return nil
}

func validateTimePricing(p *TimePricing) ([]timePricingRange, error) {
	if p == nil || !p.Enabled {
		return nil, nil
	}
	tz := strings.TrimSpace(p.Timezone)
	if tz == "" {
		return nil, infraerrors.BadRequest("TIME_PRICING_TIMEZONE_REQUIRED", "time_pricing.timezone is required when enabled")
	}
	if _, err := time.LoadLocation(tz); err != nil {
		return nil, infraerrors.BadRequest("TIME_PRICING_INVALID_TIMEZONE", fmt.Sprintf("invalid time_pricing.timezone %q", tz))
	}
	if err := validateTimePricingLabel(p.DefaultLabel, "time_pricing.default_label", "TIME_PRICING_DEFAULT_LABEL"); err != nil {
		return nil, err
	}
	if p.DefaultMultiplier != nil && !isValidTimePricingMultiplier(*p.DefaultMultiplier) {
		return nil, infraerrors.BadRequest("TIME_PRICING_INVALID_DEFAULT_MULTIPLIER", "time_pricing.default_multiplier must be within [0,100]")
	}
	if len(p.Rules) > maxTimePricingRules {
		return nil, infraerrors.BadRequest("TIME_PRICING_TOO_MANY_RULES", fmt.Sprintf("time_pricing.rules supports at most %d rules", maxTimePricingRules))
	}
	ranges := make([]timePricingRange, 0, len(p.Rules)*2)
	for i, rule := range p.Rules {
		if err := validateTimePricingLabel(rule.Label, fmt.Sprintf("time_pricing.rules[%d].label", i), "TIME_PRICING_RULE_LABEL"); err != nil {
			return nil, err
		}
		start, err := parseStrictHHMM(rule.StartTime)
		if err != nil {
			return nil, infraerrors.BadRequest("TIME_PRICING_INVALID_START", fmt.Sprintf("time_pricing.rules[%d].start_time: %v", i, err))
		}
		end, err := parseStrictHHMM(rule.EndTime)
		if err != nil {
			return nil, infraerrors.BadRequest("TIME_PRICING_INVALID_END", fmt.Sprintf("time_pricing.rules[%d].end_time: %v", i, err))
		}
		if start == end {
			return nil, infraerrors.BadRequest("TIME_PRICING_EMPTY_RANGE", fmt.Sprintf("time_pricing.rules[%d] start_time and end_time must differ", i))
		}
		if !isValidTimePricingMultiplier(rule.Multiplier) {
			return nil, infraerrors.BadRequest("TIME_PRICING_INVALID_MULTIPLIER", fmt.Sprintf("time_pricing.rules[%d].multiplier must be within [0,100]", i))
		}
		if start < end {
			ranges = append(ranges, timePricingRange{start: start, end: end, ruleIndex: i})
		} else {
			ranges = append(ranges,
				timePricingRange{start: start, end: 24 * 60, ruleIndex: i},
				timePricingRange{start: 0, end: end, ruleIndex: i},
			)
		}
	}
	for i := 0; i < len(ranges); i++ {
		for j := i + 1; j < len(ranges); j++ {
			if ranges[i].overlaps(ranges[j]) {
				return nil, infraerrors.BadRequest("TIME_PRICING_OVERLAP",
					fmt.Sprintf("time_pricing rules[%d] and rules[%d] overlap", ranges[i].ruleIndex, ranges[j].ruleIndex))
			}
		}
	}
	return ranges, nil
}

type timePricingRange struct {
	start     int
	end       int
	ruleIndex int
}

func (r timePricingRange) overlaps(other timePricingRange) bool {
	return r.start < other.end && other.start < r.end
}

func parseStrictHHMM(value string) (int, error) {
	if !strictHHMMPattern.MatchString(value) {
		return 0, fmt.Errorf("format must be HH:MM")
	}
	hour := int(value[0]-'0')*10 + int(value[1]-'0')
	minute := int(value[3]-'0')*10 + int(value[4]-'0')
	if hour > 23 || minute > 59 {
		return 0, fmt.Errorf("value must be a valid 24-hour time")
	}
	return hour*60 + minute, nil
}

func (p *TimePricing) MultiplierAt(now time.Time) AppliedTimePricing {
	if p == nil || !p.Enabled {
		return AppliedTimePricing{Multiplier: 1}
	}
	loc, err := time.LoadLocation(strings.TrimSpace(p.Timezone))
	if err != nil {
		return AppliedTimePricing{Multiplier: 1}
	}
	local := now.In(loc)
	minute := local.Hour()*60 + local.Minute()
	for i := range p.Rules {
		rule := &p.Rules[i]
		start, errStart := parseStrictHHMM(rule.StartTime)
		end, errEnd := parseStrictHHMM(rule.EndTime)
		if errStart != nil || errEnd != nil || start == end || !isValidTimePricingMultiplier(rule.Multiplier) {
			continue
		}
		if minuteInTimePricingRange(minute, start, end) {
			cp := *rule
			return AppliedTimePricing{
				Multiplier: rule.Multiplier,
				Timezone:   strings.TrimSpace(p.Timezone),
				Label:      strings.TrimSpace(rule.Label),
				Rule:       &cp,
			}
		}
	}
	return AppliedTimePricing{
		Multiplier: p.defaultMultiplier(),
		Timezone:   strings.TrimSpace(p.Timezone),
		Label:      strings.TrimSpace(p.DefaultLabel),
	}
}

func validateTimePricingLabel(label, field, code string) error {
	label = strings.TrimSpace(label)
	if label == "" {
		return infraerrors.BadRequest(code+"_REQUIRED", field+" is required when time_pricing is enabled")
	}
	if utf8.RuneCountInString(label) > maxTimePricingLabelRunes {
		return infraerrors.BadRequest(code+"_TOO_LONG", fmt.Sprintf("%s supports at most %d characters", field, maxTimePricingLabelRunes))
	}
	return nil
}

func (p *TimePricing) defaultMultiplier() float64 {
	if p == nil || p.DefaultMultiplier == nil || !isValidTimePricingMultiplier(*p.DefaultMultiplier) {
		return 1
	}
	return *p.DefaultMultiplier
}

func isValidTimePricingMultiplier(multiplier float64) bool {
	return !math.IsNaN(multiplier) && !math.IsInf(multiplier, 0) && multiplier >= 0 && multiplier <= 100
}

func minuteInTimePricingRange(minute, start, end int) bool {
	if start < end {
		return minute >= start && minute < end
	}
	return minute >= start || minute < end
}

func effectivePricingAt(now time.Time) time.Time {
	if now.IsZero() {
		return timezone.Now()
	}
	return now
}
