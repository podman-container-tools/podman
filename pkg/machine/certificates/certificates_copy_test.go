package certificates

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGuestCopyCommand(t *testing.T) {
	const anchor = "/etc/pki/ca-trust/source/anchors"
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{"plain", "/home/user/cert.pem", "sudo cp /home/user/cert.pem " + anchor},
		{"space", "/home/First Last/cert.pem", "sudo cp '/home/First Last/cert.pem' " + anchor},
		{"parenthesis", "/home/First(Last/cert.pem", "sudo cp '/home/First(Last/cert.pem' " + anchor},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, guestCopyCommand(tt.source))
		})
	}
}
