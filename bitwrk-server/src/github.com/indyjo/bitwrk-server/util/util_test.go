package util

import (
	"testing"
)

func TestStripPort(t *testing.T) {
	expect := func(argument, expected string) {
		actual := StripPort(argument)
		if actual != expected {
			t.Errorf("Expected: stripPort(%#v) = %#v  --  Got: %#v", argument, expected, actual)
		}
	}

	expect("127.0.0.1", "127.0.0.1")
	expect("127.0.0.1:8082", "127.0.0.1")
	expect("2a01:4f8:141:322c::2", "2a01:4f8:141:322c::2")
	expect("[2a01:4f8:141:322c::2]:8082", "2a01:4f8:141:322c::2")
}
