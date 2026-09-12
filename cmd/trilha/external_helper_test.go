package main

import (
	"errors"
	"os/exec"
)

func errorAs(err error, target **exec.ExitError) bool { return errors.As(err, target) }
