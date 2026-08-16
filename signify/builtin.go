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

type builtinKey struct {
	name   string
	key    string // a signify public key file, verbatim
	scopes []string
}

// A set rather than a single key so rotation works: a new key ships in one
// release and signs from the next, while the outgoing one stays valid.
// Selection is by key number, so it never surfaces to users. Names are dated
// so a rotation adds a key rather than shadowing one.
var builtinKeys = []builtinKey{
	{
		name: "plakar-20260815",
		key: "untrusted comment: plakar signify public key\n" +
			"RWT9B2Rj2tbKp89csIacV+QlvjDB4EHX9rAmS2pqgSXBbFovoLruzuSo\n",
		scopes: []string{
			// The parent of community/ and enterprise/, so one key
			// covers the whole dist tree.
			"https://plakar.io/dist/plugins/kloset/",

			// A genuinely signed artifact is no less trustworthy
			// for having been carried in by hand.
			LocalScope,
		},
	},
}
