package web

import (
	"crypto/md5" // #nosec G501 -- Gravatar requires MD5 per their protocol
	"fmt"
	"strings"
)

func GravatarURL(email string, size int) string {
	email = strings.TrimSpace(email)
	email = strings.ToLower(email)
	hash := md5.Sum([]byte(email)) // #nosec G401 -- Gravatar requires MD5 per their protocol
	return fmt.Sprintf("https://www.gravatar.com/avatar/%x?d=identicon&s=%d", hash, size)
}
