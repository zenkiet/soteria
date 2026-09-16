package wails

import "testing"

func TestLinkRoute(t *testing.T) {
	cases := map[string]string{
		"soteria://files/POS/Trung%20TR/DLL": "/files/POS/Trung%20TR/DLL",
		"soteria://settings":                 "",
		"http://192.168.1.194:8000/dav/POS":  "",
		"-psn_0_12345":                       "",
	}
	for in, want := range cases {
		if got := linkRoute(in); got != want {
			t.Errorf("linkRoute(%q) = %q, want %q", in, got, want)
		}
	}
}
