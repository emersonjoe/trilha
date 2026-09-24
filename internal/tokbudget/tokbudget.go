// Package tokbudget is the one estimator the tools share: how many tokens a
// piece of text roughly costs. It is deliberately dumb and public — four
// characters per token, rounded up — because its job is to budget an output
// (a map, an AGENTS.md, a slice of docs), not to bill anybody. Every output
// that prints a number from here labels it "est.".
package tokbudget

// CharsPerToken is the exchange rate: four characters of Go-flavored English
// and Markdown per token. It is a constant so the tools that budget with it
// stay comparable, and a test pins the contract: change it and every "est."
// number in every output moves with it, on purpose.
const CharsPerToken = 4

// Estimate returns the token cost of s, rounded up. Empty is zero.
func Estimate(s string) int {
	return (len(s) + CharsPerToken - 1) / CharsPerToken
}
