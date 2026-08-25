package main

import "testing"

func TestParseConfigAddressPrecedence(t *testing.T) {
	configuration, err := parseConfig(nil, func(key string) string {
		if key == "PORT" {
			return "19123"
		}
		return ""
	})
	if err != nil {
		t.Fatal(err)
	}
	if configuration.address != "127.0.0.1:19123" {
		t.Fatalf("address = %s", configuration.address)
	}
	configuration, err = parseConfig([]string{"-addr=127.0.0.1:19234"}, func(string) string { return "19123" })
	if err != nil {
		t.Fatal(err)
	}
	if configuration.address != "127.0.0.1:19234" {
		t.Fatalf("explicit address = %s", configuration.address)
	}
}

func TestParseConfigRejectsUnsafeAddressAndPort(t *testing.T) {
	if _, err := parseConfig([]string{"-addr=0.0.0.0:19081"}, func(string) string { return "" }); err == nil {
		t.Fatal("non-loopback address was accepted")
	}
	if _, err := parseConfig(nil, func(key string) string {
		if key == "PORT" {
			return "not-a-port"
		}
		return ""
	}); err == nil {
		t.Fatal("invalid PORT was accepted")
	}
}
