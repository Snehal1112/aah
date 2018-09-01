// Copyright (c) Jeevanandam M. (https://github.com/jeevatkm)
// Source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package settings

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"aahframe.work/aah/ahttp"
	"aahframe.work/aah/config"
	"aahframe.work/aah/essentials"
	"aahframe.work/aah/internal/util"
	"aahframe.work/aah/log"
)

// Constants
const (
	DefaultEnvProfile       = "dev"
	DefaultHTTPPort         = "8080"
	DefaultSecureJSONPrefix = ")]}',\n"
	ProfilePrefix           = "env."
)

// Settings represents parsed/infered config values for the application.
type Settings struct {
	PhysicalPathMode       bool
	PackagedMode           bool
	ServerHeaderEnabled    bool
	RequestIDEnabled       bool
	SSLEnabled             bool
	LetsEncryptEnabled     bool
	GzipEnabled            bool
	SecureHeadersEnabled   bool
	AccessLogEnabled       bool
	StaticAccessLogEnabled bool
	DumpLogEnabled         bool
	Initialized            bool
	HotReload              bool
	AuthSchemeExists       bool
	Redirect               bool
	Pid                    int
	HTTPMaxHdrBytes        int
	ImportPath             string
	BaseDir                string
	Type                   string
	EnvProfile             string
	SSLCert                string
	SSLKey                 string
	ServerHeader           string
	RequestIDHeaderKey     string
	SecureJSONPrefix       string
	ShutdownGraceTimeStr   string
	DefaultContentType     string
	HTTPReadTimeout        time.Duration
	HTTPWriteTimeout       time.Duration
	ShutdownGraceTimeout   time.Duration

	cfg *config.Config
}

// SetProfile method is to set application environment profile setting.
func (s *Settings) SetProfile(p string) error {
	if !strings.HasPrefix(p, ProfilePrefix) {
		p = ProfilePrefix + p
	}
	if err := s.cfg.SetProfile(p); err != nil {
		return err
	}
	s.EnvProfile = strings.TrimPrefix(p, ProfilePrefix)
	return nil
}

// Refresh method to parse/infer config values and populate settings instance.
func (s *Settings) Refresh(cfg *config.Config) error {
	s.cfg = cfg

	var err error
	s.SetProfile(s.cfg.StringDefault("env.active", DefaultEnvProfile))
	s.SSLEnabled = s.cfg.BoolDefault("server.ssl.enable", false)
	s.LetsEncryptEnabled = s.cfg.BoolDefault("server.ssl.lets_encrypt.enable", false)
	s.Redirect = s.cfg.BoolDefault("server.redirect.enable", false)

	readTimeout := s.cfg.StringDefault("server.timeout.read", "90s")
	writeTimeout := s.cfg.StringDefault("server.timeout.write", "90s")
	if !util.IsValidTimeUnit(readTimeout, "s", "m") || !util.IsValidTimeUnit(writeTimeout, "s", "m") {
		return errors.New("'server.timeout.{read|write}' value is not a valid time unit")
	}

	if s.HTTPReadTimeout, err = time.ParseDuration(readTimeout); err != nil {
		return fmt.Errorf("'server.timeout.read': %s", err)
	}

	if s.HTTPWriteTimeout, err = time.ParseDuration(writeTimeout); err != nil {
		return fmt.Errorf("'server.timeout.write': %s", err)
	}

	maxHdrBytesStr := s.cfg.StringDefault("server.max_header_bytes", "1mb")
	if maxHdrBytes, er := ess.StrToBytes(maxHdrBytesStr); er == nil {
		s.HTTPMaxHdrBytes = int(maxHdrBytes)
	} else {
		return errors.New("'server.max_header_bytes' value is not a valid size unit")
	}

	s.SSLCert = cfg.StringDefault("server.ssl.cert", "")
	s.SSLKey = cfg.StringDefault("server.ssl.key", "")
	if err = s.checkSSLConfigValues(); err != nil {
		return err
	}

	s.Type = s.cfg.StringDefault("type", "")
	if s.Type != "websocket" {
		if _, err = ess.StrToBytes(s.cfg.StringDefault("request.max_body_size", "5mb")); err != nil {
			return errors.New("'request.max_body_size' value is not a valid size unit")
		}

		s.ServerHeader = s.cfg.StringDefault("server.header", "")
		s.ServerHeaderEnabled = !ess.IsStrEmpty(s.ServerHeader)
		s.RequestIDEnabled = s.cfg.BoolDefault("request.id.enable", true)
		s.RequestIDHeaderKey = s.cfg.StringDefault("request.id.header", ahttp.HeaderXRequestID)
		s.SecureHeadersEnabled = s.cfg.BoolDefault("security.http_header.enable", true)
		s.GzipEnabled = s.cfg.BoolDefault("render.gzip.enable", true)
		s.AccessLogEnabled = s.cfg.BoolDefault("server.access_log.enable", false)
		s.StaticAccessLogEnabled = s.cfg.BoolDefault("server.access_log.static_file", true)
		s.DumpLogEnabled = s.cfg.BoolDefault("server.dump_log.enable", false)
		if rd := s.cfg.StringDefault("render.default", ""); len(rd) > 0 {
			s.DefaultContentType = util.MimeTypeByExtension("some." + rd)
		}

		s.SecureJSONPrefix = s.cfg.StringDefault("render.secure_json.prefix", DefaultSecureJSONPrefix)

		ahttp.GzipLevel = s.cfg.IntDefault("render.gzip.level", 4)
		if !(ahttp.GzipLevel >= 1 && ahttp.GzipLevel <= 9) {
			return fmt.Errorf("'render.gzip.level' is not a valid level value: %v", ahttp.GzipLevel)
		}
	}

	s.ShutdownGraceTimeStr = s.cfg.StringDefault("server.timeout.grace_shutdown", "60s")
	if !util.IsValidTimeUnit(s.ShutdownGraceTimeStr, "s", "m") {
		log.Warn("'server.timeout.grace_shutdown' value is not a valid time unit, assigning default value 60s")
		s.ShutdownGraceTimeStr = "60s"
	}
	s.ShutdownGraceTimeout, _ = time.ParseDuration(s.ShutdownGraceTimeStr)

	return nil
}

func (s *Settings) checkSSLConfigValues() error {
	if s.SSLEnabled {
		if !s.LetsEncryptEnabled && (ess.IsStrEmpty(s.SSLCert) || ess.IsStrEmpty(s.SSLKey)) {
			return errors.New("SSL config is incomplete; either enable 'server.ssl.lets_encrypt.enable' or provide 'server.ssl.cert' & 'server.ssl.key' value")
		} else if !s.LetsEncryptEnabled {
			if !ess.IsFileExists(s.SSLCert) {
				return fmt.Errorf("SSL cert file not found: %s", s.SSLCert)
			}

			if !ess.IsFileExists(s.SSLKey) {
				return fmt.Errorf("SSL key file not found: %s", s.SSLKey)
			}
		}
	}

	if s.LetsEncryptEnabled && !s.SSLEnabled {
		return errors.New("let's encrypt enabled, however SSL 'server.ssl.enable' is not enabled for application")
	}
	return nil
}
