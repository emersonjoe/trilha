// Package cookbook holds the code behind the Cookbook section of the site.
// Every Go block of those pages is a declaration copied from a file here,
// and a site test checks that the two still match: a recipe that stops
// compiling stops being documented.
//
// Nothing in the framework imports this package. It exists to be built by
// `go vet ./...` and read by a person, so it uses the standard library only
// — a database driver, a password hash and a metrics client all live outside
// it, and the pages say where.
package cookbook
