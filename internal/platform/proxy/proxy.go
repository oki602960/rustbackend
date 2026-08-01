package proxy

import (
	"net/http"
	"net/http/httputil"
	"net/url"
)

func New(target string) (http.Handler, error) {
	upstream, err := url.Parse(target)
	if err != nil {
		return nil, err
	}

	return httputil.NewSingleHostReverseProxy(upstream), nil
}
