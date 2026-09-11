package altcha

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"
)

// A challenge created, solved and verified by the PHP library the API uses
// (altcha-org/altcha 2.1), so this test checks the Go solver against the
// server's implementation rather than against itself.
const phpChallenge = `{"parameters":{"algorithm":"PBKDF2/SHA-256","cost":10,"expiresAt":4102444800,"keyLength":32,"keyPrefix":"74ea8ff470ea31d12435bf3afd6c092c","keySignature":"83bc002378f93868bdbc74e7321a247094f33d427702d1ba9c65fc538f2f44a1","nonce":"00112233445566778899aabbccddeeff","salt":"ffeeddccbbaa99887766554433221100"},"signature":"0532970d1b5d18025b652d4b4137920573149908fa14b7d94c451eb1b1482779"}`

const (
	phpCounter    = 37
	phpDerivedKey = "74ea8ff470ea31d12435bf3afd6c092cd6726038976342cf3a8fa57770aa55e8"
)

func TestSolveMatchesThePHPLibrary(t *testing.T) {
	result, err := Solve(context.Background(), json.RawMessage(phpChallenge))
	if err != nil {
		t.Fatal(err)
	}
	if result.Counter != phpCounter {
		t.Fatalf("counter = %d, want %d", result.Counter, phpCounter)
	}

	raw, err := base64.StdEncoding.DecodeString(result.Payload)
	if err != nil {
		t.Fatalf("payload is not base64: %v", err)
	}

	var payload struct {
		Challenge json.RawMessage `json:"challenge"`
		Solution  struct {
			Counter    int    `json:"counter"`
			DerivedKey string `json:"derivedKey"`
		} `json:"solution"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}

	if payload.Solution.DerivedKey != phpDerivedKey {
		t.Errorf("derivedKey = %s, want %s", payload.Solution.DerivedKey, phpDerivedKey)
	}
	// Verbatim, byte for byte: the server re-derives its signature from it.
	if string(payload.Challenge) != phpChallenge {
		t.Errorf("challenge was not echoed verbatim:\n got %s\nwant %s", payload.Challenge, phpChallenge)
	}
}

func TestSolveRefusesAnUnknownAlgorithm(t *testing.T) {
	_, err := Solve(context.Background(), json.RawMessage(`{"parameters":{"algorithm":"SCRYPT","cost":1,"keyLength":32,"keyPrefix":"00","nonce":"00","salt":"00"}}`))
	if err == nil {
		t.Fatal("expected an error for an unsupported algorithm")
	}
}

func TestSolveStopsWhenCancelled(t *testing.T) {
	// A prefix no derived key will start with inside the deadline.
	impossible := `{"parameters":{"algorithm":"PBKDF2/SHA-256","cost":1000,"keyLength":32,"keyPrefix":"0000000000000000","nonce":"00","salt":"00"}}`

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	if _, err := Solve(ctx, json.RawMessage(impossible)); err == nil {
		t.Fatal("expected the solver to stop at the deadline")
	}
}
