/*
 * Copyright (c) 2025 Mathieu Masson <mathieu@plakar.io>
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

package cached

import (
	"runtime"

	"github.com/PlakarKorp/kloset/repository"
	"github.com/PlakarKorp/plakar/appcontext"
	"github.com/PlakarKorp/plakar/subcommands"
	"github.com/google/uuid"
)

func init() {
	if runtime.GOOS != "windows" {
		subcommands.Register(func() subcommands.Subcommand { return &CachedReq{} },
			subcommands.BeforeRepositoryOpen, "cachedreq")
	}
}

type CachedReq struct {
	subcommands.SubcommandBase

	RepoID uuid.UUID
}

func (cmd *CachedReq) Parse(ctx *appcontext.AppContext, args []string) error {
	panic("Not implemented")
}

func (cmd *CachedReq) Close() error {
	panic("Not implemented")
}

func (cmd *CachedReq) Execute(ctx *appcontext.AppContext, repo *repository.Repository) (int, error) {
	panic("Not implemented")
}
