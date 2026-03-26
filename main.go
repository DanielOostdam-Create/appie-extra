package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	appie "github.com/gwillem/appie-go"
)

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

func printJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(v)
}

func configPath() string {
	if p := os.Getenv("APPIE_CONFIG"); p != "" {
		return p
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "appie", "config.json")
}

func mustAuth(ctx context.Context) *appie.Client {
	cp := configPath()
	client, err := appie.NewWithConfig(cp)
	if err != nil {
		fatal("Not authenticated. Run: appie login\nError: %v", err)
	}
	if !client.IsAuthenticated() {
		fatal("Not authenticated. Run: appie login")
	}
	return client
}

func usage() {
	fmt.Fprintf(os.Stderr, `appie-extra — additional commands for Albert Heijn

Usage: appie-extra <command> [args...]

Commands:
  member                         Show member profile and segmentation data
  previously-bought [size] [page] Get purchase history (default: size=50, page=0)
  search-recipes [query] [size]  Search Allerhande recipes (default: size=10)
  recipe <id>                    Get full recipe with ingredients and steps
  bonus-products [limit]         Get all current bonus products (default: 50)
  bonus-spotlight                Get featured/highlighted bonus products
  bonusbox [next|YYYY-MM-DD]     Show personal Bonus Box offers (default: this week)
  add-freetext <text> [qty]      Add free-text item to shopping list
  batch-add                      Add multiple items from stdin JSON
  list-to-order <list-id>        Convert shopping list to active order
  order-summary                  Show order pricing totals
  clear-order                    Empty the active order
  fulfillments                   Show scheduled deliveries
  koopzegels                     Show koopzegels (stamp) balance and savings
  brabantia                      Show Brabantia spaaractie status and balance
  delivery-slots                 Show available delivery time slots

Config: uses same tokens as appie CLI (~/.config/appie/config.json)
`)
	os.Exit(1)
}

// GraphQL helper — uses the client's authenticated HTTP
func doGraphQL(ctx context.Context, client *appie.Client, query string, result any) error {
	return client.DoGraphQL(ctx, query, nil, result)
}

// ---- Commands ----

func cmdMember(ctx context.Context) {
	client := mustAuth(ctx)
	member, err := client.GetMember(ctx)
	if err != nil {
		fatal("GetMember failed: %v", err)
	}
	printJSON(member)
}

func cmdPreviouslyBought(ctx context.Context, args []string) {
	client := mustAuth(ctx)

	size := 50
	page := 0
	if len(args) > 0 {
		size, _ = strconv.Atoi(args[0])
	}
	if len(args) > 1 {
		page, _ = strconv.Atoi(args[1])
	}

	query := fmt.Sprintf(`{
		productSearch(input: {
			query: ""
			previouslyBought: true
			size: %d
			page: %d
		}) {
			products {
				id
				title
				brand
				category
			}
			page {
				totalElements
				totalPages
			}
		}
	}`, size, page)

	var result struct {
		ProductSearch struct {
			Products []json.RawMessage `json:"products"`
			Page struct {
				TotalElements int `json:"totalElements"`
				TotalPages    int `json:"totalPages"`
			} `json:"page"`
		} `json:"productSearch"`
	}

	if err := doGraphQL(ctx, client, query, &result); err != nil {
		fatal("Previously bought query failed: %v", err)
	}

	printJSON(map[string]any{
		"products":      result.ProductSearch.Products,
		"totalElements": result.ProductSearch.Page.TotalElements,
		"totalPages":    result.ProductSearch.Page.TotalPages,
		"page":          page,
		"size":          size,
	})
}

func cmdSearchRecipes(ctx context.Context, args []string) {
	client := mustAuth(ctx)

	queryText := ""
	size := 10
	if len(args) > 0 {
		queryText = args[0]
	}
	if len(args) > 1 {
		size, _ = strconv.Atoi(args[1])
	}

	// Sanitize user input to prevent GraphQL injection (size is safe as int)
	if size <= 0 || size > 100 {
		size = 10
	}
	sanitizedQuery := strings.ReplaceAll(queryText, `"`, ``)
	sanitizedQuery = strings.ReplaceAll(sanitizedQuery, `\`, ``)
	sanitizedQuery = strings.ReplaceAll(sanitizedQuery, "\n", " ")

	var query string
	if sanitizedQuery != "" {
		query = fmt.Sprintf(`{ recipeSearch(query: { ingredients: "%s", size: %d }) { result { id title slug } } }`, sanitizedQuery, size)
	} else {
		query = fmt.Sprintf(`{ recipeSearch(query: { size: %d }) { result { id title slug } } }`, size)
	}

	var result struct {
		RecipeSearch struct {
			Result []json.RawMessage `json:"result"`
		} `json:"recipeSearch"`
	}

	if err := client.DoGraphQL(ctx, query, nil, &result); err != nil {
		fatal("Recipe search failed: %v", err)
	}

	printJSON(map[string]any{
		"recipes": result.RecipeSearch.Result,
		"total":   len(result.RecipeSearch.Result),
	})
}

func cmdRecipe(ctx context.Context, args []string) {
	if len(args) < 1 {
		fatal("Usage: appie-extra recipe <id>")
	}
	client := mustAuth(ctx)

	id, err := strconv.Atoi(args[0])
	if err != nil {
		fatal("Invalid recipe ID: %s", args[0])
	}

	// Verified fields: id, title, slug, description, cookTime, servings { number }, ingredients { text quantity name { singular plural } }
	query := fmt.Sprintf(`{
		recipe(id: %d) {
			id
			title
			description
			cookTime
			servings { number }
			ingredients {
				text
				quantity
				name { singular plural }
			}
		}
	}`, id)

	var result struct {
		Recipe json.RawMessage `json:"recipe"`
	}

	if err := doGraphQL(ctx, client, query, &result); err != nil {
		fatal("Get recipe failed: %v", err)
	}

	// Output the raw recipe JSON
	os.Stdout.Write(result.Recipe)
	fmt.Println()
}

func cmdBonusProducts(ctx context.Context, args []string) {
	client := mustAuth(ctx)

	products, err := client.GetBonusProducts(ctx)
	if err != nil {
		fatal("GetBonusProducts failed: %v", err)
	}

	limit := 50
	if len(args) > 0 {
		limit, _ = strconv.Atoi(args[0])
	}
	if limit > len(products) {
		limit = len(products)
	}

	printJSON(map[string]any{
		"products": products[:limit],
		"total":    len(products),
	})
}

func cmdBonusSpotlight(ctx context.Context) {
	client := mustAuth(ctx)

	products, err := client.GetSpotlightBonusProducts(ctx)
	if err != nil {
		fatal("GetSpotlightBonusProducts failed: %v", err)
	}

	printJSON(map[string]any{
		"products": products,
		"total":    len(products),
	})
}

func cmdBonusBox(ctx context.Context, args []string) {
	client := mustAuth(ctx)

	var bonusDate string
	if len(args) > 0 {
		// Allow explicit date or "next" keyword
		if args[0] == "next" {
			now := time.Now()
			daysSinceSun := int(now.Weekday())
			bonusDate = now.AddDate(0, 0, -daysSinceSun+7).Format("2006-01-02")
		} else {
			bonusDate = args[0]
		}
	} else {
		// Current week: find most recent Sunday
		now := time.Now()
		daysSinceSun := int(now.Weekday())
		bonusDate = now.AddDate(0, 0, -daysSinceSun).Format("2006-01-02")
	}

	var result json.RawMessage
	ep := "/mobile-services/bonuspage/v1/personal?bonusStartDate=" + bonusDate
	if err := client.DoRequest(ctx, "GET", ep, nil, &result); err != nil {
		fatal("Bonusbox request failed (date: %s): %v", bonusDate, err)
	}

	printJSON(result)
}

func cmdAddFreetext(ctx context.Context, args []string) {
	if len(args) < 1 {
		fatal("Usage: appie-extra add-freetext <text> [quantity]")
	}
	client := mustAuth(ctx)

	text := args[0]
	qty := 1
	if len(args) > 1 {
		qty, _ = strconv.Atoi(args[1])
		if qty <= 0 {
			qty = 1
		}
	}
	err := client.AddFreeTextToShoppingList(ctx, text, qty)
	if err != nil {
		fatal("AddFreeTextToShoppingList failed: %v", err)
	}

	printJSON(map[string]any{"ok": true, "text": text, "quantity": qty})
}

func cmdBatchAdd(ctx context.Context) {
	client := mustAuth(ctx)

	var items []struct {
		ID       int    `json:"id"`
		Qty      int    `json:"qty"`
		Text     string `json:"text"`
	}

	if err := json.NewDecoder(os.Stdin).Decode(&items); err != nil {
		fatal("Invalid JSON input: %v", err)
	}

	added := 0
	for _, item := range items {
		if item.Text != "" {
			q := item.Qty
			if q <= 0 {
				q = 1
			}
			if err := client.AddFreeTextToShoppingList(ctx, item.Text, q); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to add freetext '%s': %v\n", item.Text, err)
				continue
			}
		} else if item.ID > 0 {
			qty := item.Qty
			if qty <= 0 {
				qty = 1
			}
			if err := client.AddProductToShoppingList(ctx, item.ID, qty); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to add product %d: %v\n", item.ID, err)
				continue
			}
		}
		added++
	}

	printJSON(map[string]any{"ok": true, "added": added, "total": len(items)})
}

func cmdKoopzegels(ctx context.Context) {
	client := mustAuth(ctx)

	query := `{
		purchaseStampBalance {
			points { currentBookletPoints fullBooklets totalPoints }
			money { invested { amount } interest { amount } payout { amount } }
			constants {
				price { amount }
				partialBookletTarget { points interest { amount } }
				fullBookletTarget { points interest { amount } }
			}
		}
		purchaseStampSavingGoal { target: amount { amount } name }
	}`

	var result json.RawMessage
	if err := doGraphQL(ctx, client, query, &result); err != nil {
		fatal("Koopzegels query failed: %v", err)
	}

	printJSON(result)
}

func cmdBrabantia(ctx context.Context) {
	client := mustAuth(ctx)

	// First get program details
	programQuery := `query FetchLoyaltyProgram($programId: Int!) {
		loyaltyProgram(programId: $programId) {
			id name type status
			savingPeriod { start end }
			redeemPeriod { start end }
			content { title }
		}
	}`

	var programResult json.RawMessage
	if err := client.DoGraphQL(ctx, programQuery, map[string]any{"programId": 217, "withProducts": false}, &programResult); err != nil {
		fatal("Brabantia program query failed: %v", err)
	}

	// Then get balance
	balanceQuery := `query FetchLoyaltyPointsBalance($programIds: [Int!]!) {
		loyaltyPointsBalances(programIds: $programIds) { programId balance }
	}`

	var balanceResult json.RawMessage
	if err := client.DoGraphQL(ctx, balanceQuery, map[string]any{"programIds": []int{217}}, &balanceResult); err != nil {
		fatal("Brabantia balance query failed: %v", err)
	}

	printJSON(map[string]any{
		"program": programResult,
		"balance": balanceResult,
	})
}

func loadDeliveryAddress() map[string]any {
	home, _ := os.UserHomeDir()
	cfgPath := filepath.Join(home, "ah-assistant", "config.json")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		fatal("Cannot read config.json for delivery address: %v", err)
	}
	var cfg struct {
		DeliveryAddress *struct {
			City        string `json:"city"`
			CountryCode string `json:"country_code"`
			HouseNumber int    `json:"house_number"`
			PostalCode  string `json:"postal_code"`
			Street      string `json:"street"`
		} `json:"delivery_address"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		fatal("Cannot parse config.json: %v", err)
	}
	if cfg.DeliveryAddress == nil {
		fatal("No delivery_address in ~/ah-assistant/config.json. Add it with: city, country_code, house_number, postal_code, street")
	}
	return map[string]any{
		"city":        cfg.DeliveryAddress.City,
		"countryCode": cfg.DeliveryAddress.CountryCode,
		"houseNumber": cfg.DeliveryAddress.HouseNumber,
		"postalCode":  cfg.DeliveryAddress.PostalCode,
		"street":      cfg.DeliveryAddress.Street,
	}
}

func cmdDeliverySlots(ctx context.Context) {
	client := mustAuth(ctx)

	query := `query FetchDeliveryOrderSlotDays($address: MemberAddressInput!) {
		orderDeliverySlots(address: $address) {
			date isFullyBooked
			slots {
				startTime endTime
				deliveryLocationId shiftCode
				isFullyBooked
				serviceCharge { defaultPrice { amount } price { amount } }
				nudgeType
			}
		}
	}`

	vars := map[string]any{
		"address": loadDeliveryAddress(),
	}

	var result json.RawMessage
	if err := client.DoGraphQL(ctx, query, vars, &result); err != nil {
		fatal("Delivery slots query failed: %v", err)
	}

	printJSON(result)
}

func cmdListToOrder(ctx context.Context, args []string) {
	client := mustAuth(ctx)

	if err := client.ShoppingListToOrder(ctx); err != nil {
		fatal("ShoppingListToOrder failed: %v", err)
	}

	printJSON(map[string]any{"ok": true, "action": "shopping list converted to order"})
}

func cmdOrderSummary(ctx context.Context) {
	client := mustAuth(ctx)

	summary, err := client.GetOrderSummary(ctx)
	if err != nil {
		fatal("GetOrderSummary failed: %v", err)
	}

	printJSON(summary)
}

func cmdClearOrder(ctx context.Context) {
	client := mustAuth(ctx)

	if err := client.ClearOrder(ctx); err != nil {
		fatal("ClearOrder failed: %v", err)
	}

	printJSON(map[string]any{"ok": true, "action": "order cleared"})
}

func cmdFulfillments(ctx context.Context) {
	client := mustAuth(ctx)

	fulfillments, err := client.GetFulfillments(ctx)
	if err != nil {
		fatal("GetFulfillments failed: %v", err)
	}

	printJSON(map[string]any{
		"fulfillments": fulfillments,
		"total":        len(fulfillments),
	})
}

func main() {
	if len(os.Args) < 2 {
		usage()
	}

	ctx := context.Background()
	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "member":
		cmdMember(ctx)
	case "previously-bought":
		cmdPreviouslyBought(ctx, args)
	case "search-recipes":
		cmdSearchRecipes(ctx, args)
	case "recipe":
		cmdRecipe(ctx, args)
	case "bonus-products":
		cmdBonusProducts(ctx, args)
	case "bonus-spotlight":
		cmdBonusSpotlight(ctx)
	case "bonusbox", "bonus-box":
		cmdBonusBox(ctx, args)
	case "add-freetext":
		cmdAddFreetext(ctx, args)
	case "batch-add":
		cmdBatchAdd(ctx)
	case "list-to-order":
		cmdListToOrder(ctx, args)
	case "order-summary":
		cmdOrderSummary(ctx)
	case "clear-order":
		cmdClearOrder(ctx)
	case "fulfillments":
		cmdFulfillments(ctx)
	case "koopzegels", "stamps":
		cmdKoopzegels(ctx)
	case "brabantia":
		cmdBrabantia(ctx)
	case "delivery-slots", "slots":
		cmdDeliverySlots(ctx)
	case "--help", "-h", "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", cmd)
		usage()
	}
}
