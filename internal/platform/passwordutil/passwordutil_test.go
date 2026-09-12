package passwordutil

import "testing"

func TestHashAndVerify_RoundTrip(t *testing.T) {
	hash, err := Hash("correct horse battery staple")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}

	ok, err := Verify("correct horse battery staple", hash)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !ok {
		t.Fatal("expected the correct password to verify")
	}
}

func TestVerify_RejectsWrongPassword(t *testing.T) {
	hash, err := Hash("correct horse battery staple")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}

	ok, err := Verify("wrong password", hash)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if ok {
		t.Fatal("expected the wrong password to fail verification")
	}
}

func TestHash_ProducesDistinctSaltsForSamePassword(t *testing.T) {
	h1, err := Hash("same password")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	h2, err := Hash("same password")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	if h1 == h2 {
		t.Fatal("expected two hashes of the same password to differ (random salt)")
	}
}

func TestVerify_RejectsMalformedHash(t *testing.T) {
	if _, err := Verify("anything", "not-a-real-hash"); err == nil {
		t.Fatal("expected an error for a malformed hash")
	}
}
