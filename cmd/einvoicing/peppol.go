package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/JustSteveKing/einvoicing-cli/internal/api"
	"github.com/spf13/cobra"
)

func newLookupCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "lookup <scheme:identifier>",
		Short: "Can this business receive Peppol invoices?",
		Long: `Look up a Peppol participant, e.g. 9932:GB123456789 for a UK VAT
number. Both the GB-prefixed and bare forms are tried.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := a.client()
			if err != nil {
				return err
			}
			participant, err := client.Participant(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if a.jsonOut {
				return a.printJSON(participant)
			}
			if !participant.Registered {
				fmt.Fprintf(a.stdout, "%s is not registered on Peppol.\n", participant.ID)
				return nil
			}
			name := ""
			if participant.Directory != nil && participant.Directory.Name != nil {
				name = " (" + *participant.Directory.Name + ")"
			}
			fmt.Fprintf(a.stdout, "%s%s is registered on Peppol.\n", participant.ID, name)
			if len(participant.Capabilities) == 0 {
				fmt.Fprintln(a.stdout, "  It accepts no e-invoicing document types.")
			}
			for _, capability := range participant.Capabilities {
				fmt.Fprintf(a.stdout, "  accepts %s\n", capability.Name)
			}
			return nil
		},
	}
}

func newRulesetsCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "rulesets",
		Short: "List the rulesets documents are validated against",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := a.client()
			if err != nil {
				return err
			}
			rulesets, err := client.Rulesets(cmd.Context())
			if err != nil {
				return err
			}
			if a.jsonOut {
				return a.printJSON(rulesets)
			}
			w := tabwriter.NewWriter(a.stdout, 0, 2, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tSTATUS\tMANDATORY FROM")
			for _, r := range rulesets {
				fmt.Fprintf(w, "%s\t%s\t%s\n", r.ID, r.Status, r.MandatoryFrom)
			}
			return w.Flush()
		},
	}
}

func newValidateCmd(a *app) *cobra.Command {
	var ruleset string
	var failOnWarnings bool

	cmd := &cobra.Command{
		Use:   "validate <file.xml|->",
		Short: "Validate a Peppol BIS Billing 3.0 invoice or credit note",
		Long: `Validate a UBL invoice or credit note against the official Peppol
rules: the UBL schema, EN 16931 and Peppol BIS Billing 3.0. Every finding is
explained. Exits 1 when the document is invalid, so it can gate CI.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			document, err := a.readInput(args[0])
			if err != nil {
				return err
			}
			client, err := a.client()
			if err != nil {
				return err
			}
			report, err := client.Validate(cmd.Context(), document, ruleset)
			if err != nil {
				return err
			}
			if a.jsonOut {
				if err := a.printJSON(report); err != nil {
					return err
				}
			} else {
				a.printReport(report)
			}
			if !report.Valid || (failOnWarnings && report.Summary.Warnings > 0) {
				return errGate
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&ruleset, "ruleset", "", "pin a ruleset id from 'einvoicing rulesets'")
	cmd.Flags().BoolVar(&failOnWarnings, "fail-on-warnings", false, "exit 1 on warnings as well as errors")
	return cmd
}

func (a *app) printReport(report api.ValidationReport) {
	verdict := "valid"
	if !report.Valid {
		verdict = "INVALID"
	}
	fmt.Fprintf(a.stdout, "%s: %s against %s (%d errors, %d warnings)\n",
		report.Document.Type, verdict, report.Ruleset.ID, report.Summary.Errors, report.Summary.Warnings)
	a.printFindings(report.Findings)
}

func (a *app) printFindings(findings []api.Finding) {
	for _, f := range findings {
		fmt.Fprintf(a.stdout, "\n%s %s [%s]\n  %s\n", f.Severity, f.RuleID, f.Layer, f.Message)
		if f.Location.Path != nil {
			fmt.Fprintf(a.stdout, "  at %s\n", *f.Location.Path)
		} else if f.Location.XPath != nil {
			fmt.Fprintf(a.stdout, "  at %s\n", *f.Location.XPath)
		}
		fmt.Fprintf(a.stdout, "  %s\n", f.Explanation)
		if f.Fix != nil {
			fmt.Fprintf(a.stdout, "  fix: %s\n", *f.Fix)
		}
		fmt.Fprintf(a.stdout, "  %s\n", f.DocsURL)
	}
}

func newConvertCmd(a *app) *cobra.Command {
	var output, ruleset string

	cmd := &cobra.Command{
		Use:   "convert <invoice.json|->",
		Short: "Convert a JSON invoice into valid Peppol UBL",
		Long: `Convert a JSON invoice (the API's Invoice schema, or a whole
ConversionRequest) into a Peppol BIS Billing 3.0 UBL document. Totals and the
VAT breakdown are computed for you. The document is only produced if it
validates; otherwise every problem is listed and the command exits 1.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			input, err := a.readInput(args[0])
			if err != nil {
				return err
			}
			request, err := conversionRequest(input, ruleset)
			if err != nil {
				return err
			}
			client, err := a.client()
			if err != nil {
				return err
			}

			conversion, err := client.Convert(cmd.Context(), request)
			var problem *api.Problem
			if errors.As(err, &problem) && problem.IsType("invalid-invoice") {
				fmt.Fprintf(a.stdout, "The invoice cannot produce a valid Peppol document (%d problems):\n", len(problem.Findings))
				a.printFindings(problem.Findings)
				return errGate
			}
			if err != nil {
				return err
			}

			if a.jsonOut {
				return a.printJSON(conversion)
			}
			if output == "" || output == "-" {
				fmt.Fprint(a.stdout, conversion.Document)
			} else if err := os.WriteFile(output, []byte(conversion.Document), 0o644); err != nil {
				return err
			}
			fmt.Fprintf(a.stderr, "Converted: payable %s (VAT %s), validated against %s.\n",
				conversion.Totals.Payable, conversion.Totals.Tax, conversion.Validation.Ruleset.ID)
			return nil
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", "", "write the UBL document here instead of stdout")
	cmd.Flags().StringVar(&ruleset, "ruleset", "", "pin a ruleset id from 'einvoicing rulesets'")
	return cmd
}

// conversionRequest accepts either a bare Invoice or a whole
// ConversionRequest, and returns a ConversionRequest.
func conversionRequest(input []byte, ruleset string) ([]byte, error) {
	var body map[string]json.RawMessage
	if err := json.Unmarshal(bytes.TrimSpace(input), &body); err != nil {
		return nil, fmt.Errorf("the input is not a JSON object: %w", err)
	}
	if _, ok := body["invoice"]; !ok {
		body = map[string]json.RawMessage{"invoice": json.RawMessage(input)}
	}
	if _, ok := body["target"]; !ok {
		body["target"] = json.RawMessage(`"peppol-bis-billing-3"`)
	}
	if ruleset != "" {
		encoded, _ := json.Marshal(ruleset)
		body["ruleset"] = encoded
	}
	return json.Marshal(body)
}
