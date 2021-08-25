package service

import (
	"net/url"
	"testing"
)

func TestParseURL(t *testing.T) {
	u, err := url.Parse("funlp://abc_sdf:pswd_123@localhost:3323")
	if err != nil {
		t.Error(err)
		return
	}

	if u.Scheme != "funlp" {
		t.Error("scheme not equal: ", u.Scheme)
		return
	}
	if u.User.Username() != "abc_sdf" {
		t.Error("username not equal: ", u.User.Username())
		return
	}
	pswd, ok := u.User.Password()
	if !ok {
		t.Error("get password failed")
		return
	}
	if pswd != "pswd_123" {
		t.Error("password not equal: ", pswd)
		return
	}
	if u.Host != "localhost:3323" {
		t.Error("host not equal: ", u.Host)
		return
	}
}
