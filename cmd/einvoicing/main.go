// Command einvoicing is the command-line client for einvoicing.dev: sign in,
// validate and convert Peppol e-invoices, and look up participants.
//
// Peppol is a trademark of OpenPeppol AISBL. einvoicing.dev is independent
// and not affiliated with or endorsed by OpenPeppol.
package main

import (
	"errors"
	"fmt"
	"os"
	"runtime/debug"
	"strings"

	einvoicing "github.com/JustSteveKing/einvoicing-go"
)

// version is stamped at build time with -ldflags "-X main.version=...".
var version = "dev"

// go install applies no ldflags, so fall back to the module version the go
// command records in the binary.
func init() {
	if version != "dev" {
		return
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		version = strings.TrimPrefix(info.Main.Version, "v")
	}
}

// Exit codes. A document failing validation and the CLI failing are
// different outcomes, and CI needs to tell them apart: one means fix the
// invoice, the other means fix the pipeline.
const (
	exitOK    = 0
	exitGate  = 1
	exitError = 2
)

// errGate is returned when the command worked and the answer is "no": an
// invalid document, or an invoice that cannot be converted.
var errGate = errors.New("gate failed")

func main() {
	a := newApp(os.Stdin, os.Stdout, os.Stderr)
	os.Exit(a.run(os.Args[1:]))
}

func (a *app) run(args []string) int {
	cmd := newRootCmd(a)
	cmd.SetArgs(args)

	err := cmd.Execute()
	switch {
	case err == nil:
		return exitOK
	case errors.Is(err, errGate):
		return exitGate
	default:
		var problem *einvoicing.Problem
		if errors.As(err, &problem) {
			a.printProblem(problem)
		} else {
			fmt.Fprintln(a.stderr, "error:", err)
		}
		return exitError
	}
}
