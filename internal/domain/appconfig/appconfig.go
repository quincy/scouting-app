package appconfig

import (
	"context"
	"os"
)

const (
	KeyOnboardingComplete = "ONBOARDING_COMPLETE"
	KeyScoutbookOrgGUID   = "SCOUTBOOK_ORG_GUID"
	KeyUnitType           = "UNIT_TYPE"
	KeyUnitNumber         = "UNIT_NUMBER"
	KeyDefaultTimezone    = "DEFAULT_TIMEZONE"
	KeyMaxTentAgeGap      = "MAX_TENT_AGE_GAP"

	KeySMTPHost = "SMTP_HOST"
	KeySMTPPort = "SMTP_PORT"
	KeySMTPUser = "SMTP_USER"
	KeySMTPPass = "SMTP_PASS"
	KeySMTPFrom = "SMTP_FROM"
)

func GetWithHierarchy(ctx context.Context, repo Repository, envKey, configKey, defaultVal string) string {
	if v, ok := os.LookupEnv(envKey); ok {
		return v
	}
	v, err := repo.Get(ctx, configKey)
	if err != nil || v == "" {
		return defaultVal
	}
	return v
}
