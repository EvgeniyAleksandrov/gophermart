package config

import (
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"unicode"

	"github.com/jackc/pgx/v5"
)

type Validator struct {
}

func NewValidator() *Validator {
	return &Validator{}
}

func (v *Validator) Validate(cfg *Config) error {
	if cfg == nil {
		return ErrConfigNil
	}

	if err := v.validateRunAddress(cfg.RunAddress); err != nil {
		return fmt.Errorf("validate run address: %w", err)
	}

	if err := v.validateDatabaseURI(cfg.DatabaseURI); err != nil {
		return fmt.Errorf("validate database URI: %w", err)
	}

	if err := v.validateAccrualAddress(cfg.AccrualSystemAddress); err != nil {
		return fmt.Errorf("validate accrual address: %w", err)
	}

	if len(cfg.JWTSecret) < MinJWTSecretBytes {
		return fmt.Errorf("validate JWT secret: %w", ErrJWTSecretTooShort)
	}

	return nil
}

func (v *Validator) validatePort(port string) error {
	if port == "" {
		return ErrPortEmpty
	}

	if strings.IndexFunc(port, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
		return ErrPortNotNumeric
	}

	number, err := strconv.Atoi(port)
	if err != nil || number < 1 || number > 65535 {
		return ErrPortOutOfRange
	}

	return nil
}

func (v *Validator) validateRunAddress(address string) error {
	if address == "" {
		return ErrRunAddressEmpty
	}

	if strings.IndexFunc(address, unicode.IsSpace) >= 0 {
		return ErrRunAddressWhitespace
	}

	if strings.ContainsAny(address, "/?#@") {
		return ErrRunAddressFormat
	}

	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return ErrRunAddressFormat
	}

	if err := v.validateRunHost(address, host); err != nil {
		return fmt.Errorf("validate host: %w", err)
	}

	if err := v.validatePort(port); err != nil {
		return fmt.Errorf("validate port: %w", err)
	}

	return nil
}

func (v *Validator) validateRunHost(address, host string) error {
	if strings.HasPrefix(address, "[") {
		return v.validateIPv6(host, ErrRunAddressInvalidIP)
	}

	if strings.ContainsAny(host, "[]:") {
		return ErrRunAddressFormat
	}

	return nil
}

func (v *Validator) validateIPv6(host string, invalidErr error) error {
	ip, err := netip.ParseAddr(host)
	if err != nil || !ip.Is6() {
		return invalidErr
	}

	return nil
}

func (v *Validator) validateDatabaseURI(dsn string) error {
	if strings.TrimSpace(dsn) == "" {
		return ErrDatabaseURIEmpty
	}

	if err := v.validateDatabaseScheme(dsn); err != nil {
		return fmt.Errorf("validate scheme: %w", err)
	}

	if _, err := pgx.ParseConfig(dsn); err != nil {
		return fmt.Errorf("parse database URI: %w", ErrDatabaseURIInvalid)
	}

	return nil
}

func (v *Validator) validateDatabaseScheme(dsn string) error {
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		return nil
	}

	prefix, _, _ := strings.Cut(dsn, "=")
	if strings.Contains(prefix, "://") {
		return ErrDatabaseSchemeUnsupported
	}

	return nil
}

func (v *Validator) validateAccrualAddress(address string) error {
	if address == "" {
		return ErrAccrualAddressEmpty
	}

	if strings.IndexFunc(address, unicode.IsSpace) >= 0 {
		return ErrAccrualWhitespace
	}

	u, err := url.Parse(address)
	if err != nil {
		return fmt.Errorf("parse URL: %w", ErrAccrualAddressInvalid)
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return ErrAccrualScheme
	}

	if u.Opaque != "" {
		return ErrAccrualAddressInvalid
	}

	if u.Hostname() == "" {
		return ErrAccrualHostEmpty
	}

	if u.User != nil {
		return ErrAccrualCredentials
	}

	if u.EscapedPath() != "" && u.EscapedPath() != "/" {
		return ErrAccrualPath
	}

	if u.RawQuery != "" || u.ForceQuery {
		return ErrAccrualQuery
	}

	if strings.Contains(address, "#") {
		return ErrAccrualFragment
	}

	if err := v.validateAccrualHost(u); err != nil {
		return fmt.Errorf("validate host: %w", err)
	}

	if err := v.validateAccrualPort(u); err != nil {
		return fmt.Errorf("validate port: %w", err)
	}

	return nil
}

func (v *Validator) validateAccrualHost(u *url.URL) error {
	switch {
	case strings.HasPrefix(u.Host, "["):
		return v.validateIPv6(u.Hostname(), ErrAccrualInvalidIP)
	case strings.Contains(u.Hostname(), ":"):
		return ErrAccrualInvalidIP
	default:
		return nil
	}
}

func (v *Validator) validateAccrualPort(u *url.URL) error {
	if strings.HasSuffix(u.Host, ":") {
		return ErrPortEmpty
	}

	if u.Port() == "" {
		return nil
	}

	return v.validatePort(u.Port())
}
