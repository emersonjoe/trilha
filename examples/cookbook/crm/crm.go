// Package crm is one area of a server that already exists: it has its own
// app/ folder and its own package name, written by hand in this file.
// `trilha gen` follows the package it finds here and writes NewApp into the
// same one, so the binary that hosts it mounts the app with no registration
// file of its own.
package crm
