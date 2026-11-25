package setup

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"testing"
)

func TestIsCompromisedSecret(t *testing.T) {
	sum := sha256.Sum256([]byte("leaked-for-test"))
	compromisedSecretHashes[hex.EncodeToString(sum[:])] = true
	t.Cleanup(func() { delete(compromisedSecretHashes, hex.EncodeToString(sum[:])) })

	if !isCompromisedSecret("leaked-for-test") {
		t.Fatal("expected listed secret to be compromised")
	}
	if isCompromisedSecret("fresh-secret") {
		t.Fatal("unexpected match for an unlisted secret")
	}
	// Optional: GOAWAY_LEAKED_SECRET=<value from git history> checks the real entry without committing it.
	if v := os.Getenv("GOAWAY_LEAKED_SECRET"); v != "" && !isCompromisedSecret(v) {
		t.Fatal("real leaked secret not detected")
	}
}
