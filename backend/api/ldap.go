package api

import (
	"crypto/tls"
	"fmt"
	"goaway/backend/settings"
	"strings"
	"time"

	"github.com/go-ldap/ldap/v3"
)

func buildLDAPFilter(template, username string) string {
	escaped := ldap.EscapeFilter(strings.TrimSpace(username))
	if strings.TrimSpace(template) == "" {
		return fmt.Sprintf("(|(uid=%s)(sAMAccountName=%s))", escaped, escaped)
	}
	if strings.Contains(template, "%s") {
		return strings.ReplaceAll(template, "%s", escaped)
	}
	return template
}

func mapLDAPRole(memberOf []string, adminGroups []string) string {
	if len(adminGroups) == 0 {
		return "admin"
	}
	want := make(map[string]struct{}, len(adminGroups))
	for _, g := range adminGroups {
		want[strings.ToLower(strings.TrimSpace(g))] = struct{}{}
	}
	for _, entry := range memberOf {
		lower := strings.ToLower(strings.TrimSpace(entry))
		if _, ok := want[lower]; ok {
			return "admin"
		}
		// Also match CN=... prefix so operators can specify either full DN or CN name.
		if parsed, err := ldap.ParseDN(entry); err == nil && len(parsed.RDNs) > 0 && len(parsed.RDNs[0].Attributes) > 0 {
			cn := strings.ToLower(strings.TrimSpace(parsed.RDNs[0].Attributes[0].Value))
			if _, ok := want[cn]; ok {
				return "admin"
			}
		}
	}
	return "viewer"
}

func authenticateLDAP(cfg settings.LDAPConfig, username, password string) (authenticated bool, role string, connErr error) {
	if !cfg.Enabled || strings.TrimSpace(cfg.URL) == "" || strings.TrimSpace(cfg.BaseDN) == "" {
		return false, "", fmt.Errorf("LDAP is not configured")
	}

	tlsConf := &tls.Config{MinVersion: tls.VersionTLS12}
	conn, err := ldap.DialURL(
		strings.TrimSpace(cfg.URL),
		ldap.DialWithTLSConfig(tlsConf),
	)
	if err != nil {
		return false, "", fmt.Errorf("LDAP dial failed: %w", err)
	}
	defer func() { _ = conn.Close() }()
	conn.SetTimeout(5 * time.Second)

	if cfg.StartTLS {
		if err := conn.StartTLS(tlsConf); err != nil {
			return false, "", fmt.Errorf("LDAP StartTLS failed: %w", err)
		}
	}

	if cfg.BindDN != "" {
		if err := conn.Bind(cfg.BindDN, cfg.BindPassword); err != nil {
			return false, "", fmt.Errorf("LDAP service bind failed: %w", err)
		}
	}

	groupAttr := strings.TrimSpace(cfg.GroupAttribute)
	if groupAttr == "" {
		groupAttr = "memberOf"
	}

	searchReq := ldap.NewSearchRequest(
		cfg.BaseDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		2,
		5,
		false,
		buildLDAPFilter(cfg.UserFilter, username),
		[]string{"dn", groupAttr},
		nil,
	)

	res, err := conn.Search(searchReq)
	if err != nil {
		return false, "", fmt.Errorf("LDAP search failed: %w", err)
	}
	if len(res.Entries) != 1 {
		return false, "", nil
	}

	entry := res.Entries[0]
	if err := conn.Bind(entry.DN, password); err != nil {
		return false, "", nil
	}

	return true, mapLDAPRole(entry.GetAttributeValues(groupAttr), cfg.AdminGroups), nil
}
