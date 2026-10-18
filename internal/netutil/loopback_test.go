package netutil

import "testing"

func TestIsLoopbackHost(t *testing.T) {
	tests := map[string]bool{
		"localhost": true, "127.0.0.1": true, "127.1.2.3": true, "::1": true, "[::1]": true, "app.localhost": true,
		"10.0.0.5": false, "example.com": false, "0.0.0.0": false, "": false, "localhost.evil.com": false,
	}
	for host, want := range tests {
		if got := IsLoopbackHost(host); got != want {
			t.Errorf("IsLoopbackHost(%q) = %v, want %v", host, got, want)
		}
	}
}
