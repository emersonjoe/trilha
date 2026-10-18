//go:build race

package trilha

// raceDetector says the tests run under -race, where sync.Pool drops a
// share of what it is given on purpose and every allocation is instrumented:
// a test that measures memory has nothing to measure there.
const raceDetector = true
