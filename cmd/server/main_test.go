package main

import (
	"net"
	"testing"
)

func TestLocalURL(t *testing.T) {
	tests := []struct {
		name string
		addr net.Addr
		want string
	}{
		{
			name: "concrete address",
			addr: &net.TCPAddr{IP: net.ParseIP("192.0.2.10"), Port: 8080},
			want: "http://192.0.2.10:8080",
		},
		{
			name: "ipv4 wildcard",
			addr: &net.TCPAddr{IP: net.IPv4zero, Port: 8080},
			want: "http://127.0.0.1:8080",
		},
		{
			name: "ipv6 wildcard",
			addr: &net.TCPAddr{IP: net.IPv6zero, Port: 8080},
			want: "http://127.0.0.1:8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := localURL(tt.addr); got != tt.want {
				t.Errorf("localURL(%v) = %q, want %q", tt.addr, got, tt.want)
			}
		})
	}
}
