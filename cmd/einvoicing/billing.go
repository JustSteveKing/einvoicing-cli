package main

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/spf13/cobra"
)

func newUpgradeCmd(a *app) *cobra.Command {
	var plan string

	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Start a paid subscription on Stripe's hosted checkout",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := a.client()
			if err != nil {
				return err
			}
			link, err := client.CheckoutSession(cmd.Context(), plan)
			if err != nil {
				return err
			}
			fmt.Fprintf(a.stdout, "Checkout: %s\n", link.URL)
			fmt.Fprintln(a.stderr, "Your plan changes once payment is confirmed; check with 'einvoicing whoami'.")
			a.openBrowser(link.URL)
			return nil
		},
	}

	cmd.Flags().StringVar(&plan, "plan", "developer", "developer or pro")
	cmd.Flags().BoolVar(&a.noBrowser, "no-browser", false, "print the link without opening it")
	return cmd
}

func newBillingCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "billing",
		Short: "Manage your subscription, payment method and invoices on Stripe",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := a.client()
			if err != nil {
				return err
			}
			link, err := client.PortalSession(cmd.Context())
			if err != nil {
				return err
			}
			fmt.Fprintf(a.stdout, "Billing portal: %s\n", link.URL)
			a.openBrowser(link.URL)
			return nil
		},
	}

	cmd.Flags().BoolVar(&a.noBrowser, "no-browser", false, "print the link without opening it")
	return cmd
}

// openBrowser is best effort: the link is always printed first, so failing
// to open it costs nothing.
func (a *app) openBrowser(url string) {
	if a.noBrowser {
		return
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}
