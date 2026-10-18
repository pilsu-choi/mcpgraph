# Config fixtures

One directory per app. Every secret in these files is FAKE and contains the
word `FAKE` (or `Fake`) so it is obvious in review and never matches a real
credential. `internal/scan/leak_test.go` checks that none of these values
appear in any output format.
