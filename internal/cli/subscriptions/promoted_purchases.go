package subscriptions

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

// SubscriptionsPromotedPurchasesCommand returns the canonical nested promoted purchases tree.
func SubscriptionsPromotedPurchasesCommand() *ffcli.Command {
	cmd := shared.RewriteCommandTreePath(
		promotedpurchases.PromotedPurchasesCommand(),
		"asc promoted-purchases",
		"asc subscriptions promoted-purchases",
	)
	if cmd != nil {
		cmd.ShortHelp = "Manage promoted purchases for subscriptions."
		configureSubscriptionsPromotedPurchasesCreate(cmd)
	}
	return cmd
}

func configureSubscriptionsPromotedPurchasesCreate(cmd *ffcli.Command) {
	createCmd := findDirectSubcommand(cmd, "create")
	if createCmd == nil || createCmd.FlagSet == nil || createCmd.Exec == nil {
		return
	}

	createCmd.ShortUsage = "asc subscriptions promoted-purchases create --app APP_ID --product-id PRODUCT_ID --visible-for-all-users"
	createCmd.ShortHelp = "Create a promoted purchase for a subscription."
	createCmd.LongHelp = `Create a promoted purchase for a subscription.

Examples:
  asc subscriptions promoted-purchases create --app "APP_ID" --product-id "SUB_ID" --visible-for-all-users true
  asc subscriptions promoted-purchases create --app "APP_ID" --product-id "SUB_ID" --visible-for-all-users true --enabled true`

	if productTypeFlag := createCmd.FlagSet.Lookup("product-type"); productTypeFlag != nil {
		productTypeFlag.Usage = "Product type: SUBSCRIPTION (fixed for this command)"
		shared.HideFlagFromHelp(productTypeFlag)
	}
	if productIDFlag := createCmd.FlagSet.Lookup("product-id"); productIDFlag != nil {
		productIDFlag.Usage = "Subscription ID"
	}

	originalExec := createCmd.Exec
	createCmd.Exec = func(ctx context.Context, args []string) error {
		productTypeFlag := createCmd.FlagSet.Lookup("product-type")
		if productTypeFlag != nil {
			currentValue := strings.TrimSpace(productTypeFlag.Value.String())
			if currentValue == "" {
				if err := createCmd.FlagSet.Set("product-type", "SUBSCRIPTION"); err != nil {
					return err
				}
			} else if !strings.EqualFold(currentValue, "SUBSCRIPTION") {
				fmt.Fprintln(os.Stderr, "Error: --product-type is fixed to SUBSCRIPTION for this command")
				return flag.ErrHelp
			}
		}

		return originalExec(ctx, args)
	}
}

func findDirectSubcommand(cmd *ffcli.Command, name string) *ffcli.Command {
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
