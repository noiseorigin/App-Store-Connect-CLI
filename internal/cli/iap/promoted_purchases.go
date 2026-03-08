package iap

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/promotedpurchases"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// IAPPromotedPurchasesCommand returns the canonical nested promoted purchases tree.
func IAPPromotedPurchasesCommand() *ffcli.Command {
	cmd := shared.RewriteCommandTreePath(
		promotedpurchases.PromotedPurchasesCommand(),
		"asc promoted-purchases",
		"asc iap promoted-purchases",
	)
	if cmd != nil {
		cmd.ShortHelp = "Manage promoted purchases for in-app purchases."
		configureIAPPromotedPurchasesCreate(cmd)
	}
	return cmd
}

func configureIAPPromotedPurchasesCreate(cmd *ffcli.Command) {
	createCmd := findDirectIAPSubcommand(cmd, "create")
	if createCmd == nil || createCmd.FlagSet == nil || createCmd.Exec == nil {
		return
	}

	createCmd.ShortUsage = "asc iap promoted-purchases create --app APP_ID --product-id PRODUCT_ID --visible-for-all-users"
	createCmd.ShortHelp = "Create a promoted purchase for an in-app purchase."
	createCmd.LongHelp = `Create a promoted purchase for an in-app purchase.

Examples:
  asc iap promoted-purchases create --app "APP_ID" --product-id "IAP_ID" --visible-for-all-users true
  asc iap promoted-purchases create --app "APP_ID" --product-id "IAP_ID" --visible-for-all-users true --enabled true`

	if productTypeFlag := createCmd.FlagSet.Lookup("product-type"); productTypeFlag != nil {
		productTypeFlag.Usage = "Product type: IN_APP_PURCHASE (fixed for this command)"
		shared.HideFlagFromHelp(productTypeFlag)
	}
	if productIDFlag := createCmd.FlagSet.Lookup("product-id"); productIDFlag != nil {
		productIDFlag.Usage = "In-app purchase ID"
	}

	originalExec := createCmd.Exec
	createCmd.Exec = func(ctx context.Context, args []string) error {
		productTypeFlag := createCmd.FlagSet.Lookup("product-type")
		if productTypeFlag != nil {
			currentValue := strings.TrimSpace(productTypeFlag.Value.String())
			if currentValue == "" {
				if err := createCmd.FlagSet.Set("product-type", "IN_APP_PURCHASE"); err != nil {
					return err
				}
			} else if !strings.EqualFold(currentValue, "IN_APP_PURCHASE") {
				fmt.Fprintln(os.Stderr, "Error: --product-type is fixed to IN_APP_PURCHASE for this command")
				return flag.ErrHelp
			}
		}

		return originalExec(ctx, args)
	}
}

func findDirectIAPSubcommand(cmd *ffcli.Command, name string) *ffcli.Command {
	if cmd == nil {
		return nil
	}
	for _, sub := range cmd.Subcommands {
		if sub != nil && sub.Name == name {
			return sub
		}
	}
	return nil
}
