package utils

import "strings"

var allowedStaticProfileImagePaths = map[string]struct{}{
	"/user.png":           {},
	"/static/favicon.png": {},
}

func ValidateProfileImageURL(url string) bool {
	if strings.TrimSpace(url) == "" {
		return true
	}
	for _, prefix := range []string{
		"data:image/png",
		"data:image/jpeg",
		"data:image/gif",
		"data:image/webp",
	} {
		if strings.HasPrefix(url, prefix) {
			return true
		}
	}
	_, ok := allowedStaticProfileImagePaths[url]
	return ok
}
