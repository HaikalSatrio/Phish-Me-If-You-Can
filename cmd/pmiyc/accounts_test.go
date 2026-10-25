package main

import "testing"

func TestRepeatableAccountFlags(t *testing.T) {
	var flags accountFlags
	for _, value := range []string{"instagram:@ayu_ig", "github:ayu-dev", "instagram:ayu_ig"} {
		if err := flags.Set(value); err != nil {
			t.Fatal(err)
		}
	}
	if len(flags) != 2 || flags[0].Username != "ayu_ig" {
		t.Fatalf("wrong account flags: %+v", flags)
	}
	for _, invalid := range []string{"ayu", "instagram:", "github:../admin", "unknown:ayu"} {
		if err := flags.Set(invalid); err == nil || len(flags) != 2 {
			t.Fatalf("invalid clue changed flags: %s %+v", invalid, flags)
		}
	}
}
