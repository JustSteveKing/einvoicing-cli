package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/JustSteveKing/einvoicing-cli/internal/api"
	"github.com/JustSteveKing/einvoicing-cli/internal/credentials"
	"github.com/spf13/cobra"
)

type app struct {
	stdin  *bufio.Reader
	stdout io.Writer
	stderr io.Writer

	apiURL    string
	jsonOut   bool
	noBrowser bool
}

func newApp(stdin io.Reader, stdout, stderr io.Writer) *app {
	return &app{stdin: bufio.NewReader(stdin), stdout: stdout, stderr: stderr}
}

func newRootCmd(a *app) *cobra.Command {
	root := &cobra.Command{
		Use:   "einvoicing",
		Short: "Validate, convert and look up Peppol e-invoices with einvoicing.dev",
		Long: `einvoicing is the command-line client for einvoicing.dev.

Sign in once with 'einvoicing login'. In CI, set EINVOICING_API_KEY instead
and nothing is written to disk.

Exit codes: 0 success, 1 the document is invalid or cannot be converted,
2 the command itself failed.`,
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.PersistentFlags().StringVar(&a.apiURL, "api-url", "", "API base URL (default: $EINVOICING_API_URL, the saved one, or "+api.DefaultBaseURL+")")
	root.PersistentFlags().BoolVar(&a.jsonOut, "json", false, "print the API's JSON instead of a summary")

	root.AddCommand(
		newLoginCmd(a),
		newLogoutCmd(a),
		newWhoamiCmd(a),
		newUsageCmd(a),
		newKeysCmd(a),
		newLookupCmd(a),
		newRulesetsCmd(a),
		newValidateCmd(a),
		newConvertCmd(a),
		newUpgradeCmd(a),
		newBillingCmd(a),
	)

	return root
}

// baseURL: the flag, then the environment, then what was saved at login,
// then production.
func (a *app) baseURL() string {
	if a.apiURL != "" {
		return a.apiURL
	}
	if env := os.Getenv("EINVOICING_API_URL"); env != "" {
		return env
	}
	if creds, err := credentials.Load(); err == nil && creds.APIURL != "" {
		return creds.APIURL
	}
	return api.DefaultBaseURL
}

// client without a key, for signing in.
func (a *app) anonymousClient() *api.Client {
	return api.New(a.baseURL(), "", "einvoicing-cli/"+version)
}

// client with the key from EINVOICING_API_KEY or the saved credentials.
func (a *app) client() (*api.Client, error) {
	key := os.Getenv("EINVOICING_API_KEY")
	if key == "" {
		creds, err := credentials.Load()
		if err != nil {
			return nil, fmt.Errorf("reading saved credentials: %w", err)
		}
		key = creds.Key
	}
	if key == "" {
		return nil, errors.New("not signed in: run 'einvoicing login', or set EINVOICING_API_KEY")
	}
	return api.New(a.baseURL(), key, "einvoicing-cli/"+version), nil
}

func (a *app) prompt(label string) (string, error) {
	fmt.Fprint(a.stderr, label)
	line, err := a.stdin.ReadString('\n')
	if err != nil && !(errors.Is(err, io.EOF) && line != "") {
		return "", fmt.Errorf("reading input: %w", err)
	}
	return strings.TrimSpace(line), nil
}

func (a *app) printProblem(p *api.Problem) {
	fmt.Fprintf(a.stderr, "error: %s\n", p.Title)
	if p.Detail != "" {
		fmt.Fprintf(a.stderr, "  %s\n", p.Detail)
	}
	for field, messages := range p.Errors {
		for _, message := range messages {
			fmt.Fprintf(a.stderr, "  %s: %s\n", field, message)
		}
	}
	if p.RetryAfter != "" {
		fmt.Fprintf(a.stderr, "  Retry after %s seconds.\n", p.RetryAfter)
	}
}
