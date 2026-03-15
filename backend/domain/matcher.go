package domain

import (
	"regexp"
	"strings"
	"sync"
)

type Matcher struct {
	Exact     map[string]bool
	Wildcards map[string]bool
	Regexes   []*regexp.Regexp
	mu        sync.RWMutex
}

func NewMatcher() *Matcher {
	return &Matcher{
		Exact:     make(map[string]bool),
		Wildcards: make(map[string]bool),
		Regexes:   make([]*regexp.Regexp, 0),
	}
}

func (m *Matcher) Add(pattern string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if strings.HasPrefix(pattern, "*.") {
		m.Wildcards[strings.TrimPrefix(pattern, "*.")] = true
	} else if len(pattern) > 2 && pattern[0] == '/' && pattern[len(pattern)-1] == '/' {
		expr := pattern[1 : len(pattern)-1]
		if rx, err := regexp.Compile(expr); err == nil {
			m.Regexes = append(m.Regexes, rx)
		}
	} else {
		m.Exact[pattern] = true
	}
}

func (m *Matcher) GetWildcards() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	res := make([]string, 0, len(m.Wildcards))
	for w := range m.Wildcards {
		res = append(res, "*."+w)
	}
	return res
}

func (m *Matcher) GetRegexes() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	res := make([]string, 0, len(m.Regexes))
	for _, rx := range m.Regexes {
		res = append(res, "/"+rx.String()+"/")
	}
	return res
}

func (m *Matcher) AddBulk(patterns []string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, pattern := range patterns {
		if strings.HasPrefix(pattern, "*.") {
			m.Wildcards[strings.TrimPrefix(pattern, "*.")] = true
		} else if len(pattern) > 2 && pattern[0] == '/' && pattern[len(pattern)-1] == '/' {
			expr := pattern[1 : len(pattern)-1]
			if rx, err := regexp.Compile(expr); err == nil {
				m.Regexes = append(m.Regexes, rx)
			}
		} else {
			m.Exact[pattern] = true
		}
	}
}

func (m *Matcher) Match(domain string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.Exact[domain] {
		return true
	}

	parts := strings.Split(domain, ".")
	for i := 0; i < len(parts); i++ {
		sub := strings.Join(parts[i:], ".")
		if m.Wildcards[sub] {
			return true
		}
	}

	for _, rx := range m.Regexes {
		if rx.MatchString(domain) {
			return true
		}
	}

	return false
}

func (m *Matcher) Remove(pattern string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if strings.HasPrefix(pattern, "*.") {
		delete(m.Wildcards, strings.TrimPrefix(pattern, "*."))
	} else if len(pattern) > 2 && pattern[0] == '/' && pattern[len(pattern)-1] == '/' {
		expr := pattern[1 : len(pattern)-1]
		for i, rx := range m.Regexes {
			if rx.String() == expr {
				m.Regexes = append(m.Regexes[:i], m.Regexes[i+1:]...)
				break
			}
		}
	} else {
		delete(m.Exact, pattern)
	}
}
