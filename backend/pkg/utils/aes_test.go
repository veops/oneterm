package utils

import (
	"testing"
)

func TestEncryptAES(t *testing.T) {
	type args struct {
		plaintext string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Test 1",
			args: args{
				plaintext: "123456789abcdefghijklmnopqrstuvwxyz",
			},
			want: "hrr23HSXrZEOw5haacoj32QJLrHdpj42jaQcPVRf9AI8SzeSdWJhzTrYgsOgmNoN",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EncryptAES(tt.args.plaintext); got != tt.want {
				t.Errorf("EncryptAES() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDecryptAES(t *testing.T) {
	type args struct {
		cipherText string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Test 1",
			args: args{cipherText: "hrr23HSXrZEOw5haacoj32QJLrHdpj42jaQcPVRf9AI8SzeSdWJhzTrYgsOgmNoN"},
			want: "123456789abcdefghijklmnopqrstuvwxyz",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DecryptAES(tt.args.cipherText); got != tt.want {
				t.Errorf("DecryptAES() = %v, want %v", got, tt.want)
			}
		})
	}
}
