package ui

import (
	"github.com/PlakarKorp/kloset/repository"
)

type UI interface {
	SetRepository(*repository.Repository)
	Run() func()
}
