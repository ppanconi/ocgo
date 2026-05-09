package main

import (
	"os"
	"testing"
)

func TestShellCommandQuotesCookie(t *testing.T) {
	got := shellCommand([]string{
		"sudo",
		"openconnect",
		"--protocol=nc",
		"--cookie",
		"DSID=value with ' quote",
		"https://vpn.example.org/Linux",
	})

	want := `sudo openconnect --protocol=nc --cookie 'DSID=value with '"'"' quote' https://vpn.example.org/Linux`
	if got != want {
		t.Fatalf("shellCommand() = %q, want %q", got, want)
	}
}

func TestShellCommandLeavesSimpleArgumentsUnquoted(t *testing.T) {
	got := shellCommand([]string{
		"sudo",
		"openconnect",
		"--protocol=nc",
		"--cookie",
		"DSID=abc123",
		"https://vpn.example.org/Linux",
	})

	want := "sudo openconnect --protocol=nc --cookie DSID=abc123 https://vpn.example.org/Linux"
	if got != want {
		t.Fatalf("shellCommand() = %q, want %q", got, want)
	}
}

func TestContainsAnyIDMatchesWholeFieldsOnly(t *testing.T) {
	if !containsAnyID("ubuntu debian", "debian") {
		t.Fatal("containsAnyID() did not match an existing ID field")
	}
	if containsAnyID("notdebian", "debian") {
		t.Fatal("containsAnyID() matched a partial ID field")
	}
}

func TestReadOSRelease(t *testing.T) {
	path := t.TempDir() + "/os-release"
	if err := os.WriteFile(path, []byte("ID=ubuntu\nID_LIKE=\"debian\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := readOSRelease(path)
	if err != nil {
		t.Fatal(err)
	}
	if got["ID"] != "ubuntu" || got["ID_LIKE"] != "debian" {
		t.Fatalf("readOSRelease() = %#v", got)
	}
}
