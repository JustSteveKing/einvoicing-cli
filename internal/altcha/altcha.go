// Package altcha solves ALTCHA v2 proof-of-work challenges, which the
// einvoicing.dev API requires before it will email a sign-in code.
//
// The algorithm mirrors altcha-org/altcha (PHP) exactly: for counter = 0, 1,
// 2, ... derive keyLength bytes with PBKDF2, using the challenge's hash, cost
// iterations, the salt, and as the password the nonce followed by the counter
// as a 4-byte big-endian integer. The answer is the first counter whose
// derived key starts with keyPrefix.
package altcha

import (
	"bytes"
	"context"
	"crypto/pbkdf2"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"math"
	"time"
)

// Parameters are the parts of a challenge the solver needs. The challenge
// carries more (keySignature, expiresAt), which the solver ignores and the
// payload returns untouched.
type Parameters struct {
	Algorithm string `json:"algorithm"`
	Cost      int    `json:"cost"`
	KeyLength int    `json:"keyLength"`
	KeyPrefix string `json:"keyPrefix"`
	Nonce     string `json:"nonce"`
	Salt      string `json:"salt"`
}

// Result is a solved challenge.
type Result struct {
	// Payload is what the API expects as `altcha`: base64 of
	// {"challenge": <as received>, "solution": {"counter", "derivedKey"}}.
	Payload string
	Counter int
	Took    time.Duration
}

// Solve finds the counter for a challenge, exactly as received from
// POST /v1/sign-in-challenges (the `data` object).
//
// The challenge is kept as raw bytes and echoed back verbatim. The server
// verifies its signature over the parameters it parses from our payload, so
// re-serialising them (reordering keys, dropping a field this package does
// not know about) would break a correct solution.
func Solve(ctx context.Context, challenge json.RawMessage) (Result, error) {
	var envelope struct {
		Parameters Parameters `json:"parameters"`
	}
	if err := json.Unmarshal(challenge, &envelope); err != nil {
		return Result{}, fmt.Errorf("reading the challenge: %w", err)
	}
	p := envelope.Parameters

	newHash, err := hashFor(p.Algorithm)
	if err != nil {
		return Result{}, err
	}

	nonce, err := hex.DecodeString(p.Nonce)
	if err != nil {
		return Result{}, fmt.Errorf("challenge nonce is not hex: %w", err)
	}
	salt, err := hex.DecodeString(p.Salt)
	if err != nil {
		return Result{}, fmt.Errorf("challenge salt is not hex: %w", err)
	}
	prefix, err := hex.DecodeString(p.KeyPrefix)
	if err != nil {
		return Result{}, fmt.Errorf("challenge keyPrefix is not hex: %w", err)
	}
	if p.Cost < 1 || p.KeyLength < 1 {
		return Result{}, errors.New("challenge has no cost or key length")
	}

	started := time.Now()
	password := make([]byte, len(nonce)+4)
	copy(password, nonce)

	for counter := 0; counter <= math.MaxUint32; counter++ {
		if counter%64 == 0 {
			if err := ctx.Err(); err != nil {
				return Result{}, fmt.Errorf("solving the challenge: %w", err)
			}
		}

		binary.BigEndian.PutUint32(password[len(nonce):], uint32(counter))

		derived, err := pbkdf2.Key(newHash, string(password), salt, p.Cost, p.KeyLength)
		if err != nil {
			return Result{}, fmt.Errorf("deriving a key: %w", err)
		}
		if !bytes.HasPrefix(derived, prefix) {
			continue
		}

		took := time.Since(started)
		payload, err := json.Marshal(map[string]any{
			"challenge": challenge,
			"solution": map[string]any{
				"counter":    counter,
				"derivedKey": hex.EncodeToString(derived),
				"time":       took.Seconds(),
			},
		})
		if err != nil {
			return Result{}, err
		}

		return Result{Payload: base64.StdEncoding.EncodeToString(payload), Counter: counter, Took: took}, nil
	}

	return Result{}, errors.New("no solution exists for this challenge")
}

func hashFor(algorithm string) (func() hash.Hash, error) {
	switch algorithm {
	case "PBKDF2/SHA-256":
		return sha256.New, nil
	case "PBKDF2/SHA-384":
		return sha512.New384, nil
	case "PBKDF2/SHA-512":
		return sha512.New, nil
	default:
		return nil, fmt.Errorf("unsupported challenge algorithm %q", algorithm)
	}
}
