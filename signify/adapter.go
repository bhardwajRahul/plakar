/*
 * Copyright (c) 2026 Gilles Chehade <gilles@poolp.org>
 *
 * Permission to use, copy, modify, and distribute this software for any
 * purpose with or without fee is hereby granted, provided that the above
 * copyright notice and this permission notice appear in all copies.
 *
 * THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES
 * WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF
 * MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR
 * ANY SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES
 * WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN AN
 * ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT OF
 * OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.
 */

package signify

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	gosignify "github.com/PlakarKorp/go-signify"
	"github.com/PlakarKorp/pkg"
)

// PackageVerifier adapts a Verifier to the pkg module's Verifier interface.
type PackageVerifier struct {
	verifier *Verifier

	mu      sync.Mutex
	results map[string]*Result
}

func NewPackageVerifier(v *Verifier) *PackageVerifier {
	return &PackageVerifier{
		verifier: v,
		results:  make(map[string]*Result),
	}
}

// Verify implements pkg.Verifier. Errors wrap pkg.ErrUnverified so callers can
// tell a rejection from a transport failure.
func (p *PackageVerifier) Verify(artifact *pkg.Artifact, rd io.Reader) error {
	filename := artifact.Filename

	res, err := p.verifier.VerifyArtifact(artifact.Origin, filename, artifact.Signature, rd)
	if err != nil {
		return fmt.Errorf("%w: %s: %s", pkg.ErrUnverified, filename, err)
	}

	// nil when an unsigned artifact was allowed through.
	if res != nil {
		p.mu.Lock()
		p.results[filename] = res
		p.mu.Unlock()
	}

	return nil
}

func (p *PackageVerifier) Result(filename string) *Result {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.results[filename]
}

func (p *PackageVerifier) SetAllowUnsigned(allow bool) {
	p.verifier.SetAllowUnsigned(allow)
}

// Signer names the key that produced sig, by key number, so a key that has
// since been rotated out is still named. This says who signed, not that the
// signature is good.
func (p *PackageVerifier) Signer(sig []byte) string {
	if len(sig) == 0 {
		return ""
	}

	parsed, err := gosignify.ParseSignature(sig)
	if err != nil {
		return "unreadable signature"
	}

	for _, k := range p.verifier.store.Keys() {
		if k.Key.KeyNum == parsed.KeyNum {
			return k.Name
		}
	}

	return "unknown key " + parsed.KeyNum.String()
}

// BuiltinKeys returns the keys compiled into this binary. Embedded rather than
// read from disk: a key fetched from where a package could also reach would
// verify nothing.
func BuiltinKeys() []*TrustedKey {
	keys := make([]*TrustedKey, 0, len(builtinKeys))

	for _, b := range builtinKeys {
		pk, err := gosignify.ParsePublicKey([]byte(b.key))
		if err != nil {
			// A build-time mistake, not a runtime condition; the
			// tests check every builtin key parses.
			continue
		}

		scopes := make([]*Scope, 0, len(b.scopes))
		for _, s := range b.scopes {
			sc, err := ParseScope(s)
			if err != nil {
				continue
			}
			scopes = append(scopes, sc)
		}

		keys = append(keys, &TrustedKey{
			Name:    b.name,
			Key:     pk,
			Scopes:  scopes,
			Builtin: true,
		})
	}

	return keys
}

func TrustDir(configDir string) string {
	return filepath.Join(configDir, "trust")
}

// LoadTrustStore builds this run's trust store: the compiled-in keys plus any
// the user added under configDir.
func LoadTrustStore(configDir string) (*TrustStore, error) {
	store := NewTrustStore(BuiltinKeys()...)

	userKeys, err := LoadTrustDir(TrustDir(configDir))
	if err != nil {
		return nil, err
	}

	for _, k := range userKeys {
		store.Add(k)
	}

	return store, nil
}

func EnsureTrustDir(configDir string) (string, error) {
	dir := TrustDir(configDir)

	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}

	return dir, nil
}
