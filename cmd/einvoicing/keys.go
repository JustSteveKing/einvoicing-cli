package main

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

func newKeysCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "keys",
		Short: "List, create and revoke API keys (needs a live key)",
	}
	cmd.AddCommand(newKeysListCmd(a), newKeysCreateCmd(a), newKeysRevokeCmd(a))
	return cmd
}

func newKeysListCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List the account's keys, newest first",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := a.client()
			if err != nil {
				return err
			}
			keys, err := client.Keys(cmd.Context())
			if err != nil {
				return err
			}
			if a.jsonOut {
				return a.printJSON(keys)
			}
			w := tabwriter.NewWriter(a.stdout, 0, 2, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tPREFIX\tNAME\tSTATUS\tLAST USED")
			for _, key := range keys {
				status := "active"
				if key.RevokedAt != nil {
					status = "revoked"
				}
				lastUsed := "never"
				if key.LastUsedAt != nil {
					lastUsed = *key.LastUsedAt
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", key.ID, key.Prefix, key.Name, status, lastUsed)
			}
			return w.Flush()
		},
	}
}

func newKeysCreateCmd(a *app) *cobra.Command {
	var test bool
	var expiresAt string

	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a key and print its secret, the only time it is shown",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := a.client()
			if err != nil {
				return err
			}
			mode := "live"
			if test {
				mode = "test"
			}
			key, err := client.CreateKey(cmd.Context(), args[0], mode, expiresAt)
			if err != nil {
				return err
			}
			if a.jsonOut {
				return a.printJSON(key)
			}
			fmt.Fprintln(a.stdout, key.Secret)
			fmt.Fprintf(a.stderr, "Created %s (%s). Store the secret now; it cannot be shown again.\n", key.Prefix, key.Name)
			return nil
		},
	}

	cmd.Flags().BoolVar(&test, "test", false, "a test key: never metered, cannot manage keys or billing (suits CI)")
	cmd.Flags().StringVar(&expiresAt, "expires-at", "", "RFC 3339 time after which the key stops working")
	return cmd
}

func newKeysRevokeCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "revoke <key-id>",
		Short: "Revoke a key immediately",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := a.client()
			if err != nil {
				return err
			}
			if err := client.RevokeKey(cmd.Context(), args[0]); err != nil {
				return err
			}
			fmt.Fprintf(a.stdout, "Revoked %s.\n", args[0])
			return nil
		},
	}
}
