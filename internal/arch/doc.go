// Package arch holds the repository-wide architecture guards: tests that
// read the source tree and fail when a structural rule is broken. The
// rules live in .cursor/SOLID_GO.md (type, interface and package size,
// no init() registration, no mutable package state, import direction);
// solid_test.go enforces them and solid_budget.txt records the debt the
// tree carries today, which may only shrink. `make arch` runs it alone.
package arch
