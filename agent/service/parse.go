package service

import (
	"net/url"
)

func ParseAddr(str string) (protocol, addr string, err error) {
	u, err := url.Parse(str)
	if err != nil {
		return "", "", err
	}
	return u.Scheme, u.Host, nil
}
