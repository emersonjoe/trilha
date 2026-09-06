// Package crm is the module the host binary already has: the app lives inside
// it, so the generated file must be package crm and not package main.
package crm

// Mounted is what the host binary calls after NewApp().Handler().
func Mounted() bool { return true }
