package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	logger "github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

var phoneRegexp = regexp.MustCompile(`^\+[1-9]\d{6,14}$`)

// NormalizePhone 将手机号归一化为 E.164 格式。
// 目前仅支持中国大陆号码（11 位纯数字 → +86 前缀），已带 + 号的原样返回。
func NormalizePhone(phone string) (string, error) {
	phone = strings.TrimSpace(phone)
	if phone == "" {
		return "", ErrInvalidPhoneNumber
	}
	if strings.HasPrefix(phone, "+") {
		if !phoneRegexp.MatchString(phone) {
			return "", ErrInvalidPhoneNumber
		}
		return phone, nil
	}
	// 中国大陆：11 位 1 开头
	if len(phone) == 11 && strings.HasPrefix(phone, "1") {
		return "+86" + phone, nil
	}
	return "", ErrInvalidPhoneNumber
}

// SendSmsCodeForAuth 发送短信验证码（注册/登录共用入口）。
func (s *AuthService) SendSmsCodeForAuth(ctx context.Context, phone string) (*SendSmsCodeResult, error) {
	if s.settingService == nil || !s.settingService.IsPhoneRegisterEnabled(ctx) {
		return nil, ErrRegDisabled
	}
	if s.smsService == nil {
		return nil, ErrSmsNotConfigured
	}
	normalized, err := NormalizePhone(phone)
	if err != nil {
		return nil, err
	}
	res, err := s.smsService.SendVerifyCode(ctx, normalized)
	if err != nil {
		return nil, err
	}
	return &SendSmsCodeResult{Countdown: res.Countdown}, nil
}

// RegisterWithPhone 通过手机号+短信验证码注册新用户。
func (s *AuthService) RegisterWithPhone(ctx context.Context, phone, code, username, promoCode, invitationCode, affiliateCode string) (*User, error) {
	if s.settingService == nil || !s.settingService.IsPhoneRegisterEnabled(ctx) {
		return nil, ErrRegDisabled
	}
	if s.smsService == nil {
		return nil, ErrSmsNotConfigured
	}
	normalized, err := NormalizePhone(phone)
	if err != nil {
		return nil, err
	}
	if err := s.smsService.VerifyCode(ctx, normalized, code); err != nil {
		return nil, err
	}
	return s.createPhoneUser(ctx, normalized, username, promoCode, invitationCode, affiliateCode)
}

// LoginWithPhone 通过手机号+短信验证码登录。首次登录时自动注册。
func (s *AuthService) LoginWithPhone(ctx context.Context, phone, code string) (*User, error) {
	if s.settingService == nil || !s.settingService.IsPhoneRegisterEnabled(ctx) {
		return nil, ErrRegDisabled
	}
	if s.smsService == nil {
		return nil, ErrSmsNotConfigured
	}
	normalized, err := NormalizePhone(phone)
	if err != nil {
		return nil, err
	}

	// 验证码仅消耗一次
	if err := s.smsService.VerifyCode(ctx, normalized, code); err != nil {
		return nil, err
	}

	user, err := s.userRepo.GetByPhone(ctx, normalized)
	if err != nil {
		if !errors.Is(err, ErrUserNotFound) {
			return nil, fmt.Errorf("get user by phone: %w", err)
		}
		// 自动注册：用户名取后四位
		suffix := normalized
		if len(suffix) >= 4 {
			suffix = suffix[len(suffix)-4:]
		}
		return s.createPhoneUser(ctx, normalized, "user_"+suffix, "", "", "")
	}

	if !user.IsActive() {
		return nil, ErrUserNotActive
	}

	return user, nil
}

// createPhoneUser 在验证码已通过验证后创建手机号用户。
func (s *AuthService) createPhoneUser(ctx context.Context, normalizedPhone, username, promoCode, invitationCode, affiliateCode string) (*User, error) {
	exists, err := s.userRepo.ExistsByPhone(ctx, normalizedPhone)
	if err != nil {
		return nil, ErrServiceUnavailable
	}
	if exists {
		return nil, ErrPhoneAlreadyExists
	}

	var invitationRedeemCode *RedeemCode
	if s.settingService != nil && s.settingService.IsInvitationCodeEnabled(ctx) {
		if invitationCode == "" {
			return nil, ErrInvitationCodeRequired
		}
		redeemCode, err := s.redeemRepo.GetByCode(ctx, invitationCode)
		if err != nil {
			return nil, ErrInvitationCodeInvalid
		}
		if redeemCode.Type != RedeemTypeInvitation || !redeemCode.CanUse() {
			return nil, ErrInvitationCodeInvalid
		}
		invitationRedeemCode = redeemCode
	}

	grantPlan := s.resolveSignupGrantPlan(ctx, "phone")

	var defaultRPMLimit int
	if s.settingService != nil {
		defaultRPMLimit = s.settingService.GetDefaultUserRPMLimit(ctx)
	}

	syntheticEmail := "phone_" + normalizedPhone + PhoneConnectSyntheticEmailDomain
	user := &User{
		Email:        syntheticEmail,
		PasswordHash: "",
		Phone:        &normalizedPhone,
		Username:     username,
		SignupSource: "phone",
		Role:         RoleUser,
		Balance:      grantPlan.Balance,
		Concurrency:  grantPlan.Concurrency,
		RPMLimit:     defaultRPMLimit,
		Status:       StatusActive,
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		if errors.Is(err, ErrEmailExists) {
			return nil, ErrPhoneAlreadyExists
		}
		logger.LegacyPrintf("service.auth", "[Auth] Database error creating phone user: %v", err)
		return nil, ErrServiceUnavailable
	}

	s.postAuthUserBootstrap(ctx, user, "phone", true)
	s.assignSubscriptions(ctx, user.ID, grantPlan.Subscriptions, "auto assigned by signup defaults")
	_ = s.snapshotPlatformQuotaDefaults(ctx, user.ID, &grantPlan)

	if s.affiliateService != nil {
		if _, err := s.affiliateService.EnsureUserAffiliate(ctx, user.ID); err != nil {
			logger.LegacyPrintf("service.auth", "[Auth] Failed to initialize affiliate profile for phone user %d: %v", user.ID, err)
		}
		if aff := strings.TrimSpace(affiliateCode); aff != "" {
			if err := s.affiliateService.BindInviterByCode(ctx, user.ID, aff); err != nil {
				logger.LegacyPrintf("service.auth", "[Auth] Failed to bind affiliate inviter for phone user %d: %v", user.ID, err)
			}
		}
	}

	if invitationRedeemCode != nil {
		if err := s.redeemRepo.Use(ctx, invitationRedeemCode.ID, user.ID); err != nil {
			logger.LegacyPrintf("service.auth", "[Auth] Failed to mark invitation code as used for phone user %d: %v", user.ID, err)
		}
	}

	if promoCode != "" && s.promoService != nil && s.settingService != nil && s.settingService.IsPromoCodeEnabled(ctx) {
		if err := s.promoService.ApplyPromoCode(ctx, user.ID, promoCode); err != nil {
			logger.LegacyPrintf("service.auth", "[Auth] Failed to apply promo code for phone user %d: %v", user.ID, err)
		} else {
			if updatedUser, err := s.userRepo.GetByID(ctx, user.ID); err == nil {
				user = updatedUser
			}
		}
	}

	return user, nil
}
