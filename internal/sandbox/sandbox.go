package sandbox

import (
	"fmt"
	"log"
	"net"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
)

type Config struct {
	AllowedDirs   []string `json:"allowedDirs"`
	BlockedCmds   []string `json:"blockedCmds"`
	AllowedCmds   []string `json:"allowedCmds"`
	MaxOutputSize int      `json:"maxOutputSize"`
	Mode          string   `json:"mode"`
}

func DefaultConfig(projectDir string) *Config {
	return &Config{
		AllowedDirs:   []string{projectDir},
		BlockedCmds:   DefaultBlockedCmds(),
		AllowedCmds:   []string{},
		MaxOutputSize: 100 * 1024,
		Mode:          "project",
	}
}

func DefaultBlockedCmds() []string {
	return []string{
		"rm", "rmdir", "del", "format", "fdisk", "mkfs",
		"shutdown", "reboot", "halt", "poweroff",
		"passwd", "sudo", "su", "runas",
		"curl", "wget", "nc", "ncat", "netcat",
		"ssh", "scp", "sftp", "telnet",
		"dd", "mkswap", "swapon",
		"crontab", "at", "batch",
		"reg", "regedit", "regedt32",
		"chmod", "chown", "chgrp",
		"kill", "taskkill",
	}
}

func (c *Config) ValidateCommand(command string, workDir string) error {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return fmt.Errorf("empty command")
	}

	cmdName := strings.ToLower(filepath.Base(parts[0]))

	if len(c.AllowedCmds) > 0 {
		allowed := false
		for _, ac := range c.AllowedCmds {
			if strings.ToLower(ac) == cmdName {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("command %q is not in the allowed list", cmdName)
		}
	} else {
		for _, blocked := range c.BlockedCmds {
			if strings.ToLower(blocked) == cmdName {
				return fmt.Errorf("command %q is blocked for security", cmdName)
			}
		}
	}

	dangerousPatterns := []*regexp.Regexp{
		regexp.MustCompile(`rm\s+(-rf|-fr)\s+/(|\.\.|~)`),
		regexp.MustCompile(`:\(\)\{.*\}`),
		regexp.MustCompile(`>\s*/dev/sd`),
		regexp.MustCompile(`dd\s+.*of=/dev/`),
		regexp.MustCompile(`(curl|wget)\s+.*\|\s*(sh|bash|python)`),
	}

	for _, pat := range dangerousPatterns {
		if pat.MatchString(command) {
			return fmt.Errorf("command contains a dangerous pattern")
		}
	}

	shellMetachars := []string{"&", "|", ";", "$(", "`", ">", "<", "&&", "||", ">>", "<<", "\n", "\r",
		"!", "~", "{", "}", "*", "?", "\\", "'", "\"", "$",
		"|&", ">>", "<<", "<<<",
	}
	for _, mc := range shellMetachars {
		if strings.Contains(command, mc) {
			return fmt.Errorf("command contains shell metacharacter %q which is not allowed", mc)
		}
	}

	return nil
}

func (c *Config) ValidatePath(path string) error {
	if c.Mode == "unrestricted" {
		return nil
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("invalid path: %s", path)
	}

	if len(c.AllowedDirs) > 0 {
		allowed := false
		for _, dir := range c.AllowedDirs {
			absDir, err := filepath.Abs(dir)
			if err != nil {
				continue
			}
			if strings.HasPrefix(absPath, absDir+string(filepath.Separator)) || absPath == absDir {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("path %q is outside allowed directories", path)
		}
	}

	return nil
}

func ValidateURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %v", err)
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("only http/https schemes are allowed, got %q", u.Scheme)
	}

	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("empty hostname")
	}

	blockedHosts := []string{
		"localhost", "127.0.0.1", "0.0.0.0", "::1",
	}
	for _, bh := range blockedHosts {
		if host == bh {
			return fmt.Errorf("requests to %q are blocked (local address)", host)
		}
	}

	ip := net.ParseIP(host)
	if ip != nil {
		if isPrivateIP(ip) {
			return fmt.Errorf("requests to private IP %q are blocked", host)
		}
	}

	return nil
}

var privateRanges = []*net.IPNet{
	mustParseCIDR("10.0.0.0/8"),
	mustParseCIDR("172.16.0.0/12"),
	mustParseCIDR("192.168.0.0/16"),
	mustParseCIDR("169.254.0.0/16"),
	mustParseCIDR("fd00::/8"),
	mustParseCIDR("fe80::/10"),
}

func isPrivateIP(ip net.IP) bool {
	for _, r := range privateRanges {
		if r.Contains(ip) {
			return true
		}
	}
	return false
}

func mustParseCIDR(s string) *net.IPNet {
	_, network, err := net.ParseCIDR(s)
	if err != nil {
		log.Printf("WARNING: invalid CIDR %q: %v", s, err)
		return &net.IPNet{}
	}
	return network
}

func ValidateFilePath(path string, projectDir string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("invalid path: %s", path)
	}

	absProject, err := filepath.Abs(projectDir)
	if err != nil {
		return err
	}

	rel, err := filepath.Rel(absProject, absPath)
	if err != nil {
		return err
	}

	if strings.HasPrefix(rel, "..") {
		return fmt.Errorf("path %q is outside project directory", path)
	}

	return nil
}
