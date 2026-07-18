package ui

import (
	"fmt"
	"strings"
)

// Shell contains application-neutral chrome data for one rendered page.
type Shell struct {
	Language           string
	Title              string
	ProductName        string
	ProductSubtitle    string
	BrandURL           string
	BodyClass          string
	Stylesheets        []string
	AutoRefreshSeconds int
	Status             *Status
	Navigation         []NavigationGroup
	User               *User
	Labels             ShellLabels
	Header             Header
	Footer             Footer
}

// ShellLabels contains consumer-localizable text owned by the shared chrome.
// Empty fields use English defaults.
type ShellLabels struct {
	SkipToContent     string
	PrimaryNavigation string
	SignOut           string
}

func (l ShellLabels) withDefaults() ShellLabels {
	if l.SkipToContent == "" {
		l.SkipToContent = "Skip to content"
	}
	if l.PrimaryNavigation == "" {
		l.PrimaryNavigation = "Primary navigation"
	}
	if l.SignOut == "" {
		l.SignOut = "Sign out"
	}
	return l
}

type Status struct {
	Label  string
	Title  string
	Detail string
	Online bool
}

type NavigationGroup struct {
	Label       string
	AriaLabel   string
	Collapsible bool
	Count       string
	Items       []NavigationItem
}

type NavigationItem struct {
	Label          string
	URL            string
	Class          string
	Code           string
	Count          string
	CountAriaLabel string
	Active         bool
	Children       []NavigationItem
}

type User struct {
	DisplayName  string
	RoleLabel    string
	Initial      string
	AvatarURL    string
	ProfileURL   string
	LogoutURL    string
	LogoutFields []HiddenField
	Active       bool
}

// HiddenField carries application-owned form state such as a CSRF token.
type HiddenField struct{ Name, Value string }

type (
	Header struct{ Context, Title, Status string }
	Footer struct{ Primary, Secondary string }
)

func (s Shell) Validate() error {
	if strings.TrimSpace(s.Title) == "" {
		return fmt.Errorf("UI shell title is required")
	}
	if strings.TrimSpace(s.ProductName) == "" {
		return fmt.Errorf("UI shell product name is required")
	}
	if s.AutoRefreshSeconds < 0 {
		return fmt.Errorf("UI shell refresh interval must not be negative")
	}
	if err := validateSameOriginPath("brand", s.BrandURL, true); err != nil {
		return err
	}
	for _, stylesheet := range s.Stylesheets {
		if err := validateSameOriginPath("stylesheet", stylesheet, false); err != nil {
			return err
		}
	}
	if s.User != nil {
		if err := validateSameOriginPath("profile", s.User.ProfileURL, false); err != nil {
			return err
		}
		if err := validateSameOriginPath("logout", s.User.LogoutURL, false); err != nil {
			return err
		}
		for _, field := range s.User.LogoutFields {
			if strings.TrimSpace(field.Name) == "" {
				return fmt.Errorf("UI shell logout field name is required")
			}
		}
	}
	for _, group := range s.Navigation {
		for _, item := range group.Items {
			if err := validateNavigationItem(item); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateNavigationItem(item NavigationItem) error {
	if strings.TrimSpace(item.Label) == "" {
		return fmt.Errorf("UI shell navigation label is required")
	}
	if err := validateSameOriginPath("navigation", item.URL, false); err != nil {
		return fmt.Errorf("UI shell navigation URL for %q must be a same-origin absolute path", item.Label)
	}
	for _, child := range item.Children {
		if err := validateNavigationItem(child); err != nil {
			return err
		}
	}
	return nil
}

func validateSameOriginPath(kind, value string, allowEmpty bool) error {
	if allowEmpty && value == "" {
		return nil
	}
	if !strings.HasPrefix(value, "/") || strings.HasPrefix(value, "//") {
		return fmt.Errorf("UI shell %s URL must be a same-origin absolute path", kind)
	}
	return nil
}
