package linter

// RulePack defines a collection of rules that can be injected as a single unit.
// This is primarily used to inject external compliance rule sets.
type RulePack interface {
	Name() string
	Rules() []Rule
}
