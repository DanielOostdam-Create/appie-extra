# appie-extra

Additional CLI commands for [Albert Heijn](https://www.ah.nl/).

Companion tool for [gwillem/appie-go](https://github.com/gwillem/appie-go) that exposes AH API functionality not available in the official CLI. Shares the same OAuth tokens (`~/.config/appie/config.json`), so you only need to authenticate once with `appie login`.

## Commands

| Command | Description |
|---|---|
| `member` | Show member profile and segmentation data |
| `previously-bought [size] [page]` | Get full purchase history (paginated, default: 50 items) |
| `search-recipes [query] [size]` | Search Allerhande recipes by ingredient |
| `recipe <id>` | Get full recipe with ingredients, servings, and cook time |
| `bonus-products [limit]` | Get all current bonus products with full metadata |
| `bonus-spotlight` | Get featured/highlighted bonus products |
| `bonusbox [next\|YYYY-MM-DD]` | Show personal Bonus Box offers (default: current week) |
| `add-freetext <text> [qty]` | Add a free-text item to the shopping list |
| `batch-add` | Add multiple items from stdin JSON (`[{"id":123,"qty":2},{"text":"foo","qty":1}]`) |
| `list-to-order` | Convert shopping list to active order |
| `order-summary` | Show order pricing totals |
| `clear-order` | Empty the active order |
| `fulfillments` | Show scheduled deliveries |
| `koopzegels` | Show koopzegels (stamp) balance, interest, and savings goal |
| `brabantia` | Show Brabantia spaaractie status and stamp balance |
| `delivery-slots` | Show available delivery time slots |

All output is JSON.

## Prerequisites

1. Install [appie-go](https://github.com/gwillem/appie-go)
2. Authenticate: `appie login`
3. For `delivery-slots`: create `~/ah-assistant/config.json` with your delivery address:

```json
{
  "delivery_address": {
    "city": "Amsterdam",
    "country_code": "NL",
    "house_number": 1,
    "postal_code": "1000AA",
    "street": "Keizersgracht"
  }
}
```

## Installation

```bash
go install github.com/DanielOostdam-Create/appie-extra@latest
```

Or build from source:

```bash
git clone https://github.com/DanielOostdam-Create/appie-extra.git
cd appie-extra
go build -o appie-extra .
```

## Usage Examples

```bash
# Show your member profile
appie-extra member

# Browse purchase history
appie-extra previously-bought 20 0

# Search recipes by ingredient
appie-extra search-recipes "kip" 5

# Get a specific recipe
appie-extra recipe 1234567

# Check this week's personal bonus box
appie-extra bonusbox

# Check next week's bonus box
appie-extra bonusbox next

# Show koopzegels balance
appie-extra koopzegels

# Show Brabantia savings stamps
appie-extra brabantia

# Add items to shopping list in bulk
echo '[{"id":123,"qty":2},{"text":"avocado","qty":3}]' | appie-extra batch-add

# Convert shopping list to order and check pricing
appie-extra list-to-order
appie-extra order-summary

# Check available delivery slots
appie-extra delivery-slots
```

## Discovered API Endpoints

These endpoints were found by inspecting network traffic from the AH app. The AH GraphQL schema does not support introspection, so field names were discovered empirically.

### GraphQL Queries

- **Koopzegels**: `purchaseStampBalance` query -- returns points (current booklet, full booklets, total), money (invested, interest, payout), and constants (price, targets). Companion query `purchaseStampSavingGoal` for the savings goal.
- **Brabantia**: `loyaltyProgram(programId: 217)` for program details (name, status, saving/redeem periods). `loyaltyPointsBalances(programIds: [217])` for the stamp balance.
- **Delivery slots**: `orderDeliverySlots(address: MemberAddressInput!)` -- returns days with time slots, booking status, service charges, and nudge types.
- **Previously bought**: `productSearch` with `previouslyBought: true` flag.
- **Recipes**: `recipeSearch(query: { ingredients: "...", size: N })` for search. `recipe(id: N)` for full details including ingredients, servings, and cook time.

### REST Endpoints

- **Personal Bonus Box**: `GET /mobile-services/bonuspage/v1/personal?bonusStartDate=YYYY-MM-DD`

### Notes on the AH API

- GraphQL introspection is disabled -- you cannot query the schema
- Field names differ from older public documentation (e.g., openclaw)
- The API requires valid AH OAuth tokens obtained via `appie login`

## Credits

- Built on top of [gwillem/appie-go](https://github.com/gwillem/appie-go)
- Inspired by [markooms/openclaw-skill-albert-heijn](https://github.com/markooms/openclaw-skill-albert-heijn)

## License

[AGPL-3.0](LICENSE)
