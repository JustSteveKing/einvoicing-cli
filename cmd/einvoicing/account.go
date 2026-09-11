package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/JustSteveKing/einvoicing-cli/internal/api"
	"github.com/JustSteveKing/einvoicing-cli/internal/credentials"
	"github.com/spf13/cobra"
)

func newWhoamiCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "whoami",
		Short: "Show the account the current key belongs to",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := a.client()
			if err != nil {
				return err
			}
			account, err := client.Account(cmd.Context())
			if err != nil {
				return err
			}
			if a.jsonOut {
				return a.printJSON(account)
			}
			fmt.Fprintf(a.stdout, "%s (%s plan)\n", account.Email, account.Plan)
			return nil
		},
	}
}

func newUsageCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "usage",
		Short: "Show usage against your plan this billing period",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := a.client()
			if err != nil {
				return err
			}
			usage, err := client.Usage(cmd.Context())
			if err != nil {
				return err
			}
			if a.jsonOut {
				return a.printJSON(usage)
			}
			fmt.Fprintf(a.stdout, "%s plan, %s to %s\n", usage.Plan, usage.PeriodStart, usage.PeriodEnd)
			printMeter(a, "documents", usage.Documents)
			printMeter(a, "lookups", usage.Lookups)
			return nil
		},
	}
}

func printMeter(a *app, name string, m api.Meter) {
	line := fmt.Sprintf("  %-10s %d of %d", name, m.Used, m.Included)
	if m.Overage > 0 {
		line += fmt.Sprintf(" (%d over, billed at the plan's overage rate)", m.Overage)
	}
	fmt.Fprintln(a.stdout, line)
}

func newLogoutCmd(a *app) *cobra.Command {
	var keep bool

	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Revoke the saved key and forget it",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			creds, err := credentials.Load()
			if err != nil {
				return err
			}
			if creds.Key == "" {
				fmt.Fprintln(a.stdout, "Not signed in.")
				return nil
			}

			if !keep {
				if err := a.revokeSavedKey(cmd, creds.Key); err != nil {
					fmt.Fprintf(a.stderr, "warning: the key could not be revoked (%v). Revoke it with 'einvoicing keys revoke'.\n", err)
				}
			}

			if err := credentials.Delete(); err != nil {
				return err
			}
			fmt.Fprintln(a.stdout, "Signed out.")
			return nil
		},
	}

	cmd.Flags().BoolVar(&keep, "keep-key", false, "forget the key locally without revoking it")
	return cmd
}

// revokeSavedKey finds the saved key by its public prefix and revokes it.
// The prefix is the first 18 characters: einv_<mode>_<public id>.
func (a *app) revokeSavedKey(cmd *cobra.Command, secret string) error {
	if len(secret) < 18 {
		return errors.New("the saved key is not in the expected format")
	}
	client, err := a.client()
	if err != nil {
		return err
	}
	keys, err := client.Keys(cmd.Context())
	if err != nil {
		return err
	}
	for _, key := range keys {
		if key.Prefix == secret[:18] {
			return client.RevokeKey(cmd.Context(), key.ID)
		}
	}
	return errors.New("the saved key is not on this account")
}

func (a *app) printJSON(v any) error {
	encoder := json.NewEncoder(a.stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(v)
}

// readInput reads a file, or stdin when the path is "-".
func (a *app) readInput(path string) ([]byte, error) {
	if path == "-" {
		var b strings.Builder
		if _, err := a.stdin.WriteTo(&b); err != nil {
			return nil, err
		}
		return []byte(b.String()), nil
	}
	return os.ReadFile(path)
}
