GO =		go

DESTDIR =
PREFIX =	/usr/local
BINDIR =	${PREFIX}/bin
MANDIR =	${PREFIX}/man

INSTALL =	install
INSTALL_PROGRAM=${INSTALL} -m 0555
INSTALL_MAN =	${INSTALL} -m 0444

all: plakar

plakar:
	${GO} build -v .

install:
	mkdir -p ${DESTDIR}${BINDIR}
	mkdir -p ${DESTDIR}${MANDIR}/man1
	${INSTALL_PROGRAM} plakar ${DESTDIR}${BINDIR}
	find cmd/plakar -iname \*.1 -exec \
		${INSTALL_MAN} {} ${DESTDIR}${MANDIR}/man1 \;

check: test
test:
	${GO} test ./...

# coverage runs the test suite and reports total statement coverage with the
# testing/ support packages (mocks, fixtures) excluded, matching the Codecov
# `ignore` config. Go has no package-level coverage exclude, so we filter the
# profile after the fact: the `mode:` header line is kept, and every block
# belonging to a testing/ package is dropped before `go tool cover` totals it.
COVERPROFILE = coverage.out
coverage:
	${GO} test -covermode=atomic -coverprofile=${COVERPROFILE} ./...
	@grep -v '/plakar/testing/' ${COVERPROFILE} > ${COVERPROFILE}.tmp && mv ${COVERPROFILE}.tmp ${COVERPROFILE}
	@${GO} tool cover -func=${COVERPROFILE} | tail -1

.PHONY: all plakar install check test coverage
