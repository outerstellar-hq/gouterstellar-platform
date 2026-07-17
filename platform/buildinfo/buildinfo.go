// Package buildinfo exposes the immutable identity stamped into a platform build.
package buildinfo

import "strings"

var (
	buildDate   string
	buildNumber string
	commitSHA   string
)

// Info identifies one deployable platform build. Date is a UTC calendar date,
// Number is a monotonically increasing CI build number, and Commit is an
// optional short source revision.
type Info struct {
	Date     string `json:"date"`
	Number   string `json:"number"`
	Commit   string `json:"commit,omitempty"`
	Identity string `json:"identity"`
}

// Current returns the process-wide build identity injected with Go linker
// flags. Unstamped developer builds deliberately identify themselves as local.
func Current() Info {
	return resolve(buildDate, buildNumber, commitSHA)
}

func resolve(date, number, commit string) Info {
	date = strings.TrimSpace(date)
	number = strings.TrimSpace(number)
	commit = strings.TrimSpace(commit)
	if date == "" || number == "" || date == "local" || number == "dev" {
		return Info{Date: "local", Number: "dev", Identity: "local/dev"}
	}

	info := Info{
		Date:     date,
		Number:   number,
		Commit:   shortenCommit(commit),
		Identity: date + " #" + number,
	}
	if info.Commit != "" {
		info.Identity += " (" + info.Commit + ")"
	}
	return info
}

func shortenCommit(commit string) string {
	if len(commit) > 12 {
		return commit[:12]
	}
	return commit
}
