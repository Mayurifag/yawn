package git

import (
	"errors"
	"fmt"
	"strings"
)

var ErrNotHTTPSRemote = errors.New("remote URL is not HTTPS")

func IsHTTPSRemoteURL(remoteURL string) bool {
	lower := strings.ToLower(remoteURL)
	return strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "http://")
}

func ConvertHTTPSToSSH(httpsURL string) (string, error) {
	if !IsHTTPSRemoteURL(httpsURL) {
		return "", ErrNotHTTPSRemote
	}
	info, err := ParseRemoteURL(httpsURL)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("git@%s:%s/%s.git", info.Host, info.Owner, info.Repo), nil
}

var authErrorMarkers = []string{
	"permission denied",
	"authentication failed",
	"could not read username",
	"could not read password",
	"invalid credentials",
	"403 forbidden",
	"401 unauthorized",
	"support for password authentication was removed",
}

func IsAuthError(output string) bool {
	lower := strings.ToLower(output)
	for _, m := range authErrorMarkers {
		if strings.Contains(lower, m) {
			return true
		}
	}
	return false
}
