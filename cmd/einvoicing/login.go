package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/JustSteveKing/einvoicing-cli/internal/altcha"
	"github.com/JustSteveKing/einvoicing-cli/internal/api"
	"github.com/JustSteveKing/einvoicing-cli/internal/credentials"
	"github.com/spf13/cobra"
)

func newLoginCmd(a *app) *cobra.Command {
	var email, keyName string
	var test bool

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Sign in (or sign up) with a code sent to your email",
		Long: `Sign in with a one-time code sent to your email address. The first
sign-in for an address creates the account on the Free plan. There is no
password.

A proof-of-work challenge is solved first; it takes a moment and keeps the
sign-in endpoint from being abused. The new API key is saved to your config
directory, readable only by you.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()

			if email == "" {
				var err error
				if email, err = a.prompt("Email: "); err != nil {
					return err
				}
			}
			if keyName == "" {
				keyName = defaultKeyName()
			}
			mode := "live"
			if test {
				mode = "test"
			}

			client := a.anonymousClient()

			fmt.Fprint(a.stderr, "Solving the proof-of-work challenge... ")
			challenge, err := client.SignInChallenge(ctx)
			if err != nil {
				fmt.Fprintln(a.stderr)
				return err
			}
			solveCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
			defer cancel()
			solved, err := altcha.Solve(solveCtx, challenge)
			if err != nil {
				fmt.Fprintln(a.stderr)
				return err
			}
			fmt.Fprintf(a.stderr, "done (%s)\n", solved.Took.Round(10*time.Millisecond))

			signIn, err := client.RequestSignIn(ctx, email, solved.Payload)
			if err != nil {
				return err
			}

			result, err := a.confirm(ctx, client, signIn, keyName, mode)
			if err != nil {
				return err
			}

			path, err := credentials.Save(credentials.Credentials{
				APIURL: a.baseURL(),
				Key:    result.Key.Secret,
				Email:  result.Account.Email,
			})
			if err != nil {
				return fmt.Errorf("the key was created but could not be saved: %w (it is %s; store it now)", err, result.Key.Secret)
			}

			created := ""
			if result.AccountCreated {
				created = ", new account"
			}
			fmt.Fprintf(a.stdout, "Signed in as %s (%s plan%s).\n", result.Account.Email, result.Account.Plan, created)
			fmt.Fprintf(a.stdout, "Key %s (%s) saved to %s\n", result.Key.Prefix, result.Key.Name, path)
			return nil
		},
	}

	cmd.Flags().StringVar(&email, "email", "", "email address to send the code to")
	cmd.Flags().StringVar(&keyName, "key-name", "", "name for the new key (default: CLI on <hostname>)")
	cmd.Flags().BoolVar(&test, "test", false, "create a test key: never metered, cannot manage keys or billing")
	return cmd
}

// confirm asks for the code until it is accepted or the attempts run out.
func (a *app) confirm(ctx context.Context, client *api.Client, signIn api.SignIn, keyName, mode string) (api.SignInResult, error) {
	fmt.Fprintf(a.stderr, "A six-digit code was sent to %s. It expires in 10 minutes.\n", signIn.Email)

	for {
		code, err := a.prompt("Code: ")
		if err != nil {
			return api.SignInResult{}, err
		}

		result, err := client.ConfirmSignIn(ctx, signIn.ID, code, keyName, mode)
		if err == nil {
			return result, nil
		}

		var problem *api.Problem
		if !errors.As(err, &problem) {
			return api.SignInResult{}, err
		}

		switch {
		case problem.IsType("invalid-code") && problem.AttemptsRemaining != nil && *problem.AttemptsRemaining > 0:
			fmt.Fprintf(a.stderr, "That code is not right. %d attempts remain.\n", *problem.AttemptsRemaining)
		case problem.IsType("invalid-request"):
			fmt.Fprintln(a.stderr, "Enter the six digits from the email.")
		case problem.IsType("invalid-code"), problem.IsType("sign-in-expired"):
			return api.SignInResult{}, errors.New("this sign-in can no longer be used; run 'einvoicing login' again for a new code")
		default:
			return api.SignInResult{}, err
		}
	}
}

func defaultKeyName() string {
	host, err := os.Hostname()
	if err != nil || host == "" {
		return "CLI"
	}
	return "CLI on " + host
}
