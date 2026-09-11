package api

// The shapes below mirror openapi.yaml in the shared einvoicing.dev repo.
// Amounts stay strings, as the contract sends them: they are decimals, and
// the CLI never does arithmetic on them.

type SignIn struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	ExpiresAt string `json:"expires_at"`
}

type Account struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Plan      string `json:"plan"`
	CreatedAt string `json:"created_at"`
}

type APIKey struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Mode       string  `json:"mode"`
	Prefix     string  `json:"prefix"`
	CreatedAt  string  `json:"created_at"`
	LastUsedAt *string `json:"last_used_at"`
	ExpiresAt  *string `json:"expires_at"`
	RevokedAt  *string `json:"revoked_at"`
}

// NewAPIKey is a key as created: the only time its secret is ever sent.
type NewAPIKey struct {
	APIKey
	Secret string `json:"secret"`
}

type SignInResult struct {
	Account        Account   `json:"account"`
	AccountCreated bool      `json:"account_created"`
	Key            NewAPIKey `json:"key"`
}

type Meter struct {
	Used     int `json:"used"`
	Included int `json:"included"`
	Overage  int `json:"overage"`
}

type Usage struct {
	Plan        string `json:"plan"`
	PeriodStart string `json:"period_start"`
	PeriodEnd   string `json:"period_end"`
	Documents   Meter  `json:"documents"`
	Lookups     Meter  `json:"lookups"`
}

type Capability struct {
	Name           string `json:"name"`
	DocumentTypeID string `json:"document_type_id"`
	ProcessID      string `json:"process_id"`
}

type Directory struct {
	Name        *string `json:"name"`
	CountryCode *string `json:"country_code"`
}

type Participant struct {
	ID           string       `json:"id"`
	Scheme       string       `json:"scheme"`
	Identifier   string       `json:"identifier"`
	Registered   bool         `json:"registered"`
	Capabilities []Capability `json:"capabilities"`
	Directory    *Directory   `json:"directory"`
	CheckedAt    string       `json:"checked_at"`
}

type Ruleset struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Version       string `json:"version"`
	Status        string `json:"status"`
	Released      string `json:"released"`
	MandatoryFrom string `json:"mandatory_from"`
}

type Location struct {
	XPath *string `json:"xpath"`
	Line  *int    `json:"line"`
	Path  *string `json:"path"`
}

type Finding struct {
	RuleID        string   `json:"rule_id"`
	Layer         string   `json:"layer"`
	Severity      string   `json:"severity"`
	Message       string   `json:"message"`
	Explanation   string   `json:"explanation"`
	Fix           *string  `json:"fix"`
	BusinessTerms []string `json:"business_terms"`
	Location      Location `json:"location"`
	DocsURL       string   `json:"docs_url"`
}

type LayerResult struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

type ValidationReport struct {
	Valid   bool `json:"valid"`
	Ruleset struct {
		ID      string `json:"id"`
		Version string `json:"version"`
	} `json:"ruleset"`
	Document struct {
		Type string `json:"type"`
	} `json:"document"`
	Layers  []LayerResult `json:"layers"`
	Summary struct {
		Errors   int `json:"errors"`
		Warnings int `json:"warnings"`
	} `json:"summary"`
	Findings []Finding `json:"findings"`
}

type Totals struct {
	LineExtension string `json:"line_extension"`
	TaxExclusive  string `json:"tax_exclusive"`
	Tax           string `json:"tax"`
	TaxInclusive  string `json:"tax_inclusive"`
	Payable       string `json:"payable"`
}

type Conversion struct {
	Target     string           `json:"target"`
	Document   string           `json:"document"`
	Totals     Totals           `json:"totals"`
	Validation ValidationReport `json:"validation"`
}

type BillingLink struct {
	URL       string  `json:"url"`
	ExpiresAt *string `json:"expires_at"`
}
