package model

import "strings"

type Permission struct {
	Domain   string
	Action   string
	Instance string
}

func (p Permission) Implies(other Permission) bool {
	return (p.Domain == "*" || p.Domain == other.Domain) &&
		(p.Action == "*" || p.Action == other.Action) &&
		(p.Instance == "*" || p.Instance == other.Instance)
}

func (p Permission) String() string {
	if p.Instance != "*" {
		return p.Domain + ":" + p.Action + ":" + p.Instance
	}
	if p.Action != "*" {
		return p.Domain + ":" + p.Action
	}
	return p.Domain
}

func ParsePermission(s string) Permission {
	parts := strings.SplitN(s, ":", 3)
	p := Permission{Domain: parts[0]}
	if len(parts) > 1 {
		p.Action = parts[1]
	} else {
		p.Action = "*"
	}
	if len(parts) > 2 {
		p.Instance = parts[2]
	} else {
		p.Instance = "*"
	}
	return p
}
