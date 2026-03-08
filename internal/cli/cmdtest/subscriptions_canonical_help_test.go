package cmdtest

import (
	"context"
	"io"
	"strings"
	"testing"
)

func TestSubscriptionsHelpShowsCanonicalCommerceSubcommands(t *testing.T) {
	root := RootCommand("1.2.3")

	subscriptionsCmd := findSubcommand(root, "subscriptions")
	if subscriptionsCmd == nil {
		t.Fatal("expected subscriptions command")
	}
	subscriptionsUsage := subscriptionsCmd.UsageFunc(subscriptionsCmd)
	for _, expected := range []string{"pricing", "offers", "review", "promoted-purchases"} {
		if !usageListsSubcommand(subscriptionsUsage, expected) {
			t.Fatalf("expected subscriptions help to list %s, got %q", expected, subscriptionsUsage)
		}
	}
	for _, hidden := range []string{
		"prices",
		"availability",
		"price-points",
		"introductory-offers",
		"promotional-offers",
		"offer-codes",
		"win-back-offers",
		"review-screenshots",
		"app-store-review-screenshot",
		"submit",
		"promoted-purchase",
	} {
		if usageListsSubcommand(subscriptionsUsage, hidden) {
			t.Fatalf("expected subscriptions help to hide deprecated flat subcommand %s, got %q", hidden, subscriptionsUsage)
		}
	}

	groupsCmd := findSubcommand(root, "subscriptions", "groups")
	if groupsCmd == nil {
		t.Fatal("expected subscriptions groups command")
	}
	groupsUsage := groupsCmd.UsageFunc(groupsCmd)
	if usageListsSubcommand(groupsUsage, "submit") {
		t.Fatalf("expected subscriptions groups help to hide deprecated submit shim, got %q", groupsUsage)
	}

	pricingCmd := findSubcommand(root, "subscriptions", "pricing")
	if pricingCmd == nil {
		t.Fatal("expected subscriptions pricing command")
	}
	pricingUsage := pricingCmd.UsageFunc(pricingCmd)
	for _, expected := range []string{"summary", "prices", "price-points", "availability"} {
		if !usageListsSubcommand(pricingUsage, expected) {
			t.Fatalf("expected subscriptions pricing help to list %s, got %q", expected, pricingUsage)
		}
	}

	offersCmd := findSubcommand(root, "subscriptions", "offers")
	if offersCmd == nil {
		t.Fatal("expected subscriptions offers command")
	}
	offersUsage := offersCmd.UsageFunc(offersCmd)
	for _, expected := range []string{"introductory", "promotional", "offer-codes", "win-back"} {
		if !usageListsSubcommand(offersUsage, expected) {
			t.Fatalf("expected subscriptions offers help to list %s, got %q", expected, offersUsage)
		}
	}

	offerCodesCmd := findSubcommand(root, "subscriptions", "offers", "offer-codes")
	if offerCodesCmd == nil {
		t.Fatal("expected subscriptions offers offer-codes command")
	}
	offerCodesUsage := offerCodesCmd.UsageFunc(offerCodesCmd)
	if !strings.Contains(offerCodesUsage, "  generate") {
		t.Fatalf("expected subscriptions offers offer-codes help to list generate, got %q", offerCodesUsage)
	}
	if !strings.Contains(offerCodesUsage, "  values") {
		t.Fatalf("expected subscriptions offers offer-codes help to list values, got %q", offerCodesUsage)
	}

	reviewCmd := findSubcommand(root, "subscriptions", "review")
	if reviewCmd == nil {
		t.Fatal("expected subscriptions review command")
	}
	reviewUsage := reviewCmd.UsageFunc(reviewCmd)
	for _, expected := range []string{"screenshots", "app-store-screenshot", "submit", "submit-group"} {
		if !usageListsSubcommand(reviewUsage, expected) {
			t.Fatalf("expected subscriptions review help to list %s, got %q", expected, reviewUsage)
		}
	}

	promotedPurchasesCreateCmd := findSubcommand(root, "subscriptions", "promoted-purchases", "create")
	if promotedPurchasesCreateCmd == nil {
		t.Fatal("expected subscriptions promoted-purchases create command")
	}
	promotedPurchasesCreateUsage := promotedPurchasesCreateCmd.UsageFunc(promotedPurchasesCreateCmd)
	if strings.Contains(promotedPurchasesCreateUsage, "--product-type") {
		t.Fatalf("expected canonical promoted-purchases create help to hide --product-type, got %q", promotedPurchasesCreateUsage)
	}

	iapCmd := findSubcommand(root, "iap")
	if iapCmd == nil {
		t.Fatal("expected iap command")
	}
	iapUsage := iapCmd.UsageFunc(iapCmd)
	if !strings.Contains(iapUsage, "  promoted-purchases") {
		t.Fatalf("expected iap help to list promoted-purchases, got %q", iapUsage)
	}
	if usageListsSubcommand(iapUsage, "promoted-purchase") {
		t.Fatalf("expected iap help to hide deprecated singular promoted-purchase shim, got %q", iapUsage)
	}
}

func TestRemovedLegacyCommerceRootCommandsAreNotRegistered(t *testing.T) {
	root := RootCommand("1.2.3")

	for _, name := range []string{"offer-codes", "win-back-offers", "promoted-purchases"} {
		if cmd := findSubcommand(root, name); cmd != nil {
			t.Fatalf("expected removed root command %s to be absent", name)
		}
	}
}

func TestCanonicalWrapperErrorsUseCanonicalPaths(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "subscriptions offers win-back next validation",
			args:    []string{"subscriptions", "offers", "win-back", "list", "--next", "http://api.appstoreconnect.apple.com/v1/subscriptions/sub-1/winBackOffers?cursor=AQ"},
			wantErr: "subscriptions offers win-back list: --next must be an App Store Connect URL",
		},
		{
			name:    "subscriptions promoted-purchases next validation",
			args:    []string{"subscriptions", "promoted-purchases", "list", "--next", "http://api.appstoreconnect.apple.com/v1/apps/app-1/promotedPurchases?cursor=AQ"},
			wantErr: "subscriptions promoted-purchases list: --next must be an App Store Connect URL",
		},
		{
			name:    "iap promoted-purchases next validation",
			args:    []string{"iap", "promoted-purchases", "list", "--next", "http://api.appstoreconnect.apple.com/v1/apps/app-1/promotedPurchases?cursor=AQ"},
			wantErr: "iap promoted-purchases list: --next must be an App Store Connect URL",
		},
		{
			name:    "subscriptions offers offer-codes values auth error",
			args:    []string{"subscriptions", "offers", "offer-codes", "values", "--batch-id", "batch-1"},
			wantErr: "subscriptions offers offer-codes values:",
		},
		{
			name:    "subscriptions pricing prices next validation",
			args:    []string{"subscriptions", "pricing", "prices", "list", "--next", "http://api.appstoreconnect.apple.com/v1/subscriptions/sub-1/prices?cursor=AQ"},
			wantErr: "subscriptions pricing prices list: --next must be an App Store Connect URL",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			var runErr error
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				runErr = root.Run(context.Background())
			})

			if runErr == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(runErr.Error(), test.wantErr) {
				t.Fatalf("expected error %q, got %v", test.wantErr, runErr)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
		})
	}
}

func usageListsSubcommand(usage string, name string) bool {
	for _, line := range strings.Split(usage, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if fields[0] == name {
			return true
		}
	}
	return false
}
