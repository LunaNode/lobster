package lobster

import "testing"

func testWildcardMatcher(t *testing.T, regex string, s string, expect bool) {
	if wildcardMatcher(regex, s) != expect {
		t.Fatalf("wildcardMatcher(%s, %s) != %t", regex, s, expect)
	}
}

func TestWildcardMatcher(t *testing.T) {
	testWildcardMatcher(t, "", "blah", false)
	testWildcardMatcher(t, "vms", "vms", true)
	testWildcardMatcher(t, "vms", "vms/", false)
	testWildcardMatcher(t, "vms*", "vms/", true)
	testWildcardMatcher(t, "vms/*", "vms", false)
	testWildcardMatcher(t, "vms/*", "vms/blah", true)
	testWildcardMatcher(t, "*", "blah", true)
}
