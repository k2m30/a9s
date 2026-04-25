# QA User Stories: AWS Cost Explorer View (Issue #60)

Covers the Cost Explorer as a new top-level resource type with child views for
cost-by-service and daily cost breakdowns. Provides simplified cost insights --
answers to "what am I spending?", "what changed?", and "where is the money going?"

All stories are written from a black-box perspective against the design spec and
`views.yaml` configuration files.

AWS CLI equivalents are cited so testers can verify data parity.

**IMPORTANT:** The Cost Explorer API (`ce:GetCostAndUsage`, `ce:GetCostForecast`)
charges ~$0.01 per API request. This is noted throughout and the application should
warn the user on first use.

---

## A. Main Menu Integration

### A.1 Resource Type Appears in Main Menu

| ID | Story | Expected |
|----|-------|----------|
| A.1.1 | I launch a9s and the main menu is displayed. | Cost Explorer appears as a row in the resource type list. The row shows a display name (e.g., "Cost Explorer") and a dimmed shortname alias (e.g., `:costs`). |
| A.1.2 | I select the Cost Explorer entry and press Enter. | The view transitions to the Cost Overview list. A loading spinner appears while the API call is in flight. |

### A.2 Quick Access via `c` Key

| ID | Story | Expected |
|----|-------|----------|
| A.2.1 | I am on the main menu and press `c`. | The view navigates directly to the Cost Explorer view. This is a shortcut for quick access to cost data. |
| A.2.2 | I am on any resource list and press `:`. I type the shortname for costs and press Enter. | The view navigates to the Cost Explorer. |

---

## B. Cost API Warning

### B.1 First-Use Cost Warning

| ID | Story | Expected |
|----|-------|----------|
| B.1.1 | I navigate to the Cost Explorer for the first time in a session. | Before any API call is made, a warning appears (e.g., in the header or as a centered message): "Cost Explorer API charges ~$0.01/request. Press Enter to continue or Esc to cancel." The user must explicitly confirm. |
| B.1.2 | I press Enter to confirm the cost warning. | The warning disappears. The loading spinner appears. The Cost Explorer data is fetched. |
| B.1.3 | I press Esc on the cost warning. | The warning disappears. I return to the main menu (or previous view). No API call is made. No charge is incurred. |
| B.1.4 | I navigate to Cost Explorer a second time in the same session (after previously confirming). | The warning does not appear again. Cached data is displayed, or a fresh API call is made without re-prompting. The confirmation is remembered for the session. |

---

## C. Cost Overview -- Top-Level List View

### C.1 Loading State

| ID | Story | Expected |
|----|-------|----------|
| C.1.1 | I navigate to Cost Explorer and confirm the API warning. The API has not yet responded. | A spinner (animated dot) is displayed centered inside the frame. The text reads "Fetching cost data..." (or similar). The frame title shows the resource shortname with no count. |
| C.1.2 | I press keys (j, k, /, N) while the spinner is visible. | Keypresses are ignored or queued until data loads. |
| C.1.3 | The API responds successfully. | The spinner disappears. The cost overview renders with data. |
| C.1.4 | The API responds with an error (e.g., no permissions for ce:GetCostAndUsage). | The spinner disappears. A red error flash message appears in the header. |

**AWS comparison:**

```
aws ce get-cost-and-usage --time-period Start=2026-03-01,End=2026-03-27 --granularity MONTHLY --metrics UnblendedCost
```

### C.2 Cost Overview Columns

| ID | Story | Expected |
|----|-------|----------|
| C.2.1 | The cost overview renders. | The view displays columns such as: "Period" (current month or configurable period), "Total Cost" (formatted as currency, e.g., "$1,234.56"), "Forecast" (projected month-end cost), "Delta vs Last Month" (e.g., "+$123.45" or "-$50.00" or "+12%"), "Top Service" (the service with highest spend). |
| C.2.2 | The total cost matches `aws ce get-cost-and-usage` output. | The "Total Cost" value corresponds to the sum of `ResultsByTime[].Total.UnblendedCost.Amount` for the current period. |
| C.2.3 | The forecast value is present for the current month. | "Forecast" shows a projected month-end total from `GetCostForecast`. If the forecast API fails or returns no data, the field shows "n/a" or a dash. |
| C.2.4 | The delta (month-over-month comparison) is computed. | "Delta vs Last Month" shows the difference between current month total and last month total as both an absolute dollar amount and a percentage. |

**AWS comparison:**

```
aws ce get-cost-and-usage --time-period Start=2026-03-01,End=2026-03-28 --granularity MONTHLY --metrics UnblendedCost
aws ce get-cost-forecast --time-period Start=2026-03-28,End=2026-04-01 --granularity MONTHLY --metric UNBLENDED_COST
```

Expected fields visible: Period, Total Cost, Forecast, Delta vs Last Month, Top Service

### C.3 Delta Color Coding

| ID | Story | Expected |
|----|-------|----------|
| C.3.1 | The current month cost is higher than last month. | The delta value is displayed in RED (#f7768e) text, e.g., "+$234.56 (+15%)". The red color signals a cost increase. |
| C.3.2 | The current month cost is lower than last month. | The delta value is displayed in GREEN (#9ece6a) text, e.g., "-$100.00 (-8%)". Green signals a cost decrease. |
| C.3.3 | The current month cost is unchanged from last month. | The delta value is displayed in PLAIN color (#c0caf5), e.g., "$0.00 (0%)". |
| C.3.4 | This is the first month (no previous month data for comparison). | The delta column shows "n/a" or a dash. No crash. |

### C.4 Frame Title

| ID | Story | Expected |
|----|-------|----------|
| C.4.1 | Cost data loads. | The frame top border shows the title centered (e.g., "costs" or "cost-overview") with equal-length dashes on both sides. |

### C.5 Navigation

| ID | Story | Expected |
|----|-------|----------|
| C.5.1 | I press j/k to navigate rows (if multiple periods are shown, e.g., last 3 months). | The selection cursor moves between rows. |
| C.5.2 | I press g to jump to the top row, G to jump to the bottom row. | Navigation works as in other list views. |
| C.5.3 | I press h/l to scroll columns horizontally. | Hidden columns are revealed. |

### C.6 Refresh (ctrl+r)

| ID | Story | Expected |
|----|-------|----------|
| C.6.1 | I press ctrl+r on the cost overview. | The loading spinner appears. Fresh API calls are made to GetCostAndUsage and GetCostForecast. The data updates. Note: each refresh incurs API charges (~$0.01 per request). |
| C.6.2 | I had a filter active and press ctrl+r. | The data refreshes. The filter remains applied. |

### C.7 Escape (Back)

| ID | Story | Expected |
|----|-------|----------|
| C.7.1 | I press Escape on the cost overview. | I return to the main menu. |

### C.8 Help (?)

| ID | Story | Expected |
|----|-------|----------|
| C.8.1 | I press ? on the cost overview. | The help screen replaces the content inside the frame with the standard four-column layout: RESOURCE, GENERAL, NAVIGATION, HOTKEYS. |
| C.8.2 | I press any key on the help screen. | The help screen closes and the cost overview reappears. |

---

## D. Child View: Cost by Service

### D.1 Entry and Loading

| ID | Story | Expected |
|----|-------|----------|
| D.1.1 | I select a period row in the cost overview and press Enter. | The view transitions to the Cost by Service breakdown for that period. A loading spinner appears while the API call is in flight. The cost overview is pushed onto the view stack. |
| D.1.2 | The API responds successfully. | The spinner disappears. The table renders with service cost breakdown rows. The frame title updates to include the period context (e.g., "cost-by-service -- Mar 2026"). |
| D.1.3 | The API responds with an error. | A red error flash appears in the header. |

**AWS comparison:**

```
aws ce get-cost-and-usage --time-period Start=2026-03-01,End=2026-03-28 --granularity MONTHLY --metrics UnblendedCost --group-by Type=DIMENSION,Key=SERVICE
```

### D.2 Column Layout

| ID | Story | Expected |
|----|-------|----------|
| D.2.1 | The cost-by-service table renders. | Columns include: "Service" (e.g., "Amazon EC2", "Amazon S3"), "Cost" (formatted currency, e.g., "$456.78"), "% of Total" (e.g., "37%"), "Delta" (vs previous period, with color coding), "Trend" (direction indicator: up-arrow, down-arrow, or flat). |
| D.2.2 | I verify data against `aws ce get-cost-and-usage` with GROUP_BY SERVICE. | "Service" maps to `.Groups[].Keys[0]`. "Cost" maps to `.Groups[].Metrics.UnblendedCost.Amount`. Every service group returned by the CLI appears as a row. |
| D.2.3 | Services with zero cost are either hidden or shown at the bottom in DIM text. | Zero-cost services do not clutter the display. |

**AWS comparison:**

```
aws ce get-cost-and-usage --time-period Start=2026-03-01,End=2026-03-28 --granularity MONTHLY --metrics UnblendedCost --group-by Type=DIMENSION,Key=SERVICE --query 'ResultsByTime[0].Groups[].[Keys[0],Metrics.UnblendedCost.Amount]' --output table
```

Expected fields visible: Service, Cost, % of Total, Delta, Trend

### D.3 Sorting

| ID | Story | Expected |
|----|-------|----------|
| D.3.1 | The default sort is by cost descending (highest spend first). | The most expensive service appears at the top of the list. The "Cost" column header shows a sort indicator (down-arrow). |
| D.3.2 | I press N to sort by service name. | Rows sort alphabetically by service name. The sort indicator moves to the "Service" column header. |
| D.3.3 | I press A to sort by age/time. | If a time column is present, rows sort by it. Otherwise this may be a no-op. |

### D.4 Delta Color Coding

| ID | Story | Expected |
|----|-------|----------|
| D.4.1 | A service cost increased by more than 20% vs previous period. | The "Delta" column for that service is RED (#f7768e). This highlights cost anomalies. |
| D.4.2 | A service cost decreased vs previous period. | The "Delta" column for that service is GREEN (#9ece6a). |
| D.4.3 | A service cost is roughly unchanged (less than 5% change). | The "Delta" column is PLAIN color (#c0caf5). |

### D.5 Filter

| ID | Story | Expected |
|----|-------|----------|
| D.5.1 | I press / and type "EC2". | Only services whose name contains "EC2" are shown (e.g., "Amazon Elastic Compute Cloud - Compute", "EC2 - Other"). The frame title shows matched/total. |
| D.5.2 | I press Escape to clear the filter. | All services reappear. |

### D.6 Navigation and Actions

| ID | Story | Expected |
|----|-------|----------|
| D.6.1 | I press j/k/g/G/PageUp/PageDown. | Navigation works as in other list views. |
| D.6.2 | I press d on a service row. | The detail view opens showing full cost details for that service (cost breakdown by usage type, region, or other dimensions). |
| D.6.3 | I press y on a service row. | The YAML view opens showing the raw cost data as syntax-highlighted YAML. |
| D.6.4 | I press c on a service row. | The service name is copied to the clipboard. A "Copied!" flash appears. |
| D.6.5 | I press Escape. | I return to the cost overview. The cursor position is preserved. |

---

## E. Child View: Daily Cost Breakdown

### E.1 Entry and Loading

| ID | Story | Expected |
|----|-------|----------|
| E.1.1 | From the cost overview, I use a designated key (e.g., `e` or a secondary action key) to open the daily breakdown. | The view transitions to the Daily Cost Breakdown. A loading spinner appears. |
| E.1.2 | The API responds successfully. | The table renders with one row per day, showing daily cost totals. The frame title shows the period context (e.g., "daily-costs -- Mar 2026"). |

**AWS comparison:**

```
aws ce get-cost-and-usage --time-period Start=2026-03-01,End=2026-03-28 --granularity DAILY --metrics UnblendedCost
```

### E.2 Column Layout

| ID | Story | Expected |
|----|-------|----------|
| E.2.1 | The daily breakdown table renders. | Columns include: "Date" (e.g., "Mar 27", "Mar 26"), "Cost" (daily total, formatted as currency), "Delta vs Prev Day" (e.g., "+$12.34" or "-$5.00"). |
| E.2.2 | I verify data against `aws ce get-cost-and-usage` with DAILY granularity. | "Date" maps to `.ResultsByTime[].TimePeriod.Start`. "Cost" maps to `.ResultsByTime[].Total.UnblendedCost.Amount`. |
| E.2.3 | The first day of the period has no previous day to compare. | The "Delta vs Prev Day" column shows "n/a" or a dash for the first row. |

**AWS comparison:**

```
aws ce get-cost-and-usage --time-period Start=2026-03-01,End=2026-03-28 --granularity DAILY --metrics UnblendedCost --query 'ResultsByTime[].[TimePeriod.Start,Total.UnblendedCost.Amount]' --output table
```

Expected fields visible: Date, Cost, Delta vs Prev Day

### E.3 Delta Color Coding

| ID | Story | Expected |
|----|-------|----------|
| E.3.1 | Today's cost is significantly higher than yesterday. | The "Delta vs Prev Day" value is RED (#f7768e), drawing attention to the spike. |
| E.3.2 | Today's cost is lower than yesterday. | The "Delta vs Prev Day" value is GREEN (#9ece6a). |
| E.3.3 | Cost is roughly unchanged day-over-day. | The delta value is PLAIN color (#c0caf5). |

### E.4 Navigation and Actions

| ID | Story | Expected |
|----|-------|----------|
| E.4.1 | I press j/k/g/G to navigate. | Navigation works as in other list views. |
| E.4.2 | I press d on a day row. | The detail view opens showing cost details for that day (e.g., breakdown by service for that specific day). |
| E.4.3 | I press y on a day row. | The YAML view opens showing raw daily cost data. |
| E.4.4 | I press Escape. | I return to the cost overview or the parent view. |

---

## F. Cost Type Configuration

### F.1 Default Cost Metric

| ID | Story | Expected |
|----|-------|----------|
| F.1.1 | I open Cost Explorer with no custom configuration. | Cost values default to UnblendedCost. The displayed amounts match `aws ce get-cost-and-usage --metrics UnblendedCost`. |

### F.2 Configurable Cost Metric

| ID | Story | Expected |
|----|-------|----------|
| F.2.1 | I configure `costs.metric: BlendedCost` in `~/.a9s/config.yaml`. | All cost values display blended costs instead of unblended. |
| F.2.2 | I configure `costs.metric: AmortizedCost`. | All cost values display amortized costs (spreads upfront fees like RI/SP across the period). |

**AWS comparison:**

```
aws ce get-cost-and-usage --time-period Start=2026-03-01,End=2026-03-28 --granularity MONTHLY --metrics BlendedCost
aws ce get-cost-and-usage --time-period Start=2026-03-01,End=2026-03-28 --granularity MONTHLY --metrics AmortizedCost
```

### F.3 Configurable Default Period

| ID | Story | Expected |
|----|-------|----------|
| F.3.1 | No configuration. I open Cost Explorer. | The default period is the current month-to-date (MTD). |
| F.3.2 | I configure `costs.period: 7d`. | The cost overview shows data for the last 7 days. |
| F.3.3 | I configure `costs.period: 30d`. | The cost overview shows data for the last 30 days. |
| F.3.4 | I configure `costs.period: MTD`. | The cost overview shows month-to-date data. |

---

## G. Currency Formatting

### G.1 Dollar Amounts

| ID | Story | Expected |
|----|-------|----------|
| G.1.1 | The total cost for the month is $1,234.56. | The value displays as "$1,234.56" with a dollar sign, comma thousands separator, and two decimal places. |
| G.1.2 | The cost for a service is $0.02. | The value displays as "$0.02" -- small costs are not rounded to zero. |
| G.1.3 | The cost is $0.00 (zero). | The value displays as "$0.00". |
| G.1.4 | The cost is $12,345.678 (sub-cent precision from the API). | The value displays rounded to two decimal places: "$12,345.68". |
| G.1.5 | The API returns costs in a non-USD currency (e.g., EUR). | The currency unit from the API response is respected and displayed (e.g., "EUR 1,234.56"). |

---

## H. Error Handling

### H.1 Permission Errors

| ID | Story | Expected |
|----|-------|----------|
| H.1.1 | The IAM user/role does not have `ce:GetCostAndUsage` permission. | A red error flash appears in the header: "Error: AccessDeniedException" (or similar). The cost view is empty. The application remains navigable. |
| H.1.2 | The IAM user/role does not have `ce:GetCostForecast` permission but has `ce:GetCostAndUsage`. | The cost overview loads with actual cost data. The "Forecast" field shows "n/a" or a dash instead of a forecast value. A warning may appear (not a blocking error). |

### H.2 Cost Explorer Not Enabled

| ID | Story | Expected |
|----|-------|----------|
| H.2.1 | The AWS account does not have Cost Explorer enabled (first activation takes 24 hours). | A descriptive error message appears: "Cost Explorer is not enabled for this account" or similar. The user is informed that enabling it may take up to 24 hours. |

### H.3 Network and Throttling

| ID | Story | Expected |
|----|-------|----------|
| H.3.1 | The network is unreachable while fetching cost data. | A red error flash appears. The application remains responsive. |
| H.3.2 | The Cost Explorer API is throttled (rate limited). | A red error flash appears (e.g., "Error: Throttling"). The application does not retry indefinitely. |

---

## I. Caching

### I.1 Aggressive Caching to Minimize API Charges

| ID | Story | Expected |
|----|-------|----------|
| I.1.1 | I open Cost Explorer, view the data, press Escape to go back to the main menu, then navigate to Cost Explorer again. | The previously fetched cost data is displayed immediately from cache. No new API call is made. No spinner appears. |
| I.1.2 | I navigate from cost overview to cost-by-service and back. | The cost overview data is served from cache on return. No additional API charge. |
| I.1.3 | I press ctrl+r on the cost overview. | The cache is invalidated. A fresh API call is made (incurring a charge). The data updates with the latest figures. |
| I.1.4 | I switch to a different AWS profile or region and navigate to Cost Explorer. | The cache from the previous profile/region is not used. Fresh API calls are made for the new profile/region (after the cost warning). |

---

## J. Detail View

### J.1 Cost Detail (d key)

| ID | Story | Expected |
|----|-------|----------|
| J.1.1 | I select a row (period or service) and press d. | The detail view opens showing comprehensive cost information as key-value pairs. Keys are blue (#7aa2f7), values in white (#c0caf5). |
| J.1.2 | The detail view includes: period, total cost, forecast (if applicable), top services (if viewing a period), cost breakdown (if viewing a service). | All relevant cost dimensions are displayed. |
| J.1.3 | I press j/k/g/G to scroll. | Scrolling works as in other detail views. |
| J.1.4 | I press w to toggle word wrap. | Long values wrap or unwrap. |
| J.1.5 | I press c to copy. | The detail content is copied to the clipboard. |
| J.1.6 | I press Escape. | I return to the previous list view. |

### J.2 YAML View (y key)

| ID | Story | Expected |
|----|-------|----------|
| J.2.1 | I select a row and press y. | The YAML view opens showing the raw cost data as syntax-highlighted YAML. |
| J.2.2 | YAML syntax coloring is applied. | Keys blue, strings green, numbers orange, booleans purple, nulls dim. |
| J.2.3 | I press Escape. | I return to the previous list view. |

---

## K. View Stack Integration

### K.1 Navigation Stack

| ID | Story | Expected |
|----|-------|----------|
| K.1.1 | Main Menu -> Cost Overview -> Cost by Service -> Service Detail -> YAML; then Escape four times. | Each Escape pops one level: YAML -> Detail -> Cost by Service -> Cost Overview -> Main Menu. |
| K.1.2 | Main Menu -> Cost Overview -> Daily Breakdown -> Day Detail; then Escape three times. | Day Detail -> Daily Breakdown -> Cost Overview -> Main Menu. |
| K.1.3 | I navigate to Cost Explorer via `:costs` command from EC2 list. | The cost overview loads. Pressing Escape returns to the main menu. |

---

## L. Cross-Cutting Concerns

### L.1 Header Consistency

| ID | Story | Expected |
|----|-------|----------|
| L.1.1 | In every Cost Explorer view (overview, by-service, daily, detail, YAML), the header displays: "a9s" (accent bold), version (dim), profile:region (bold). | Visual inspection confirms across all cost views. |
| L.1.2 | The header right side shows "? for help" in normal mode. | Confirmed in all cost views. |

### L.2 Terminal Resize

| ID | Story | Expected |
|----|-------|----------|
| L.2.1 | I resize the terminal while viewing any cost view. | The layout reflows. Column visibility adjusts. The frame redraws correctly. |
| L.2.2 | I resize to below 60 columns. | An error message appears: "Terminal too narrow. Please resize." |
| L.2.3 | I resize to below 7 lines. | An error message appears: "Terminal too short. Please resize." |

### L.3 Help Screen

| ID | Story | Expected |
|----|-------|----------|
| L.3.1 | I press ? in any cost view. | The help screen appears with the four-column layout. Any cost-specific key bindings (e.g., `c` for quick access from main menu) are documented. |
| L.3.2 | I press any key to close help. | The cost view reappears. |

### L.4 Alternating Row Colors

| ID | Story | Expected |
|----|-------|----------|
| L.4.1 | The cost-by-service list has more than 2 rows. | Alternating rows have a subtle background color difference (#1e2030) for readability. Selected row always has blue background. |

---

## M. Demo Mode

### M.1 Synthetic Cost Fixtures

| ID | Story | Expected |
|----|-------|----------|
| M.1.1 | I launch a9s in demo mode. I navigate to Cost Explorer. | Synthetic cost data is displayed with realistic figures: multiple services, varying costs, computed deltas, a forecast value. No real AWS API calls are made. No cost warning is shown in demo mode. |
| M.1.2 | The demo data includes services with cost increases and decreases. | Some service rows show RED deltas (increases) and some show GREEN deltas (decreases), demonstrating the color-coding feature. |
| M.1.3 | I drill into cost-by-service in demo mode. | Synthetic per-service breakdown data is displayed. |
| M.1.4 | I drill into daily breakdown in demo mode. | Synthetic daily cost data is displayed with realistic day-over-day variations. |

---

## N. Multi-Account Cost Comparison (Organization Setup)

### N.1 Grouping by Account

| ID | Story | Expected |
|----|-------|----------|
| N.1.1 | The AWS account is an organization management account. I open Cost Explorer. | The cost overview may include multi-account data if configured. An option or child view exists to break down costs by linked account. |
| N.1.2 | I navigate to a cost-by-account breakdown (if available as a child view or configuration option). | Columns include: "Account" (account name or ID), "Cost", "% of Total", "Delta". Each linked account appears as a row. |

**AWS comparison:**

```
aws ce get-cost-and-usage --time-period Start=2026-03-01,End=2026-03-28 --granularity MONTHLY --metrics UnblendedCost --group-by Type=DIMENSION,Key=LINKED_ACCOUNT
```

### N.2 Single Account

| ID | Story | Expected |
|----|-------|----------|
| N.2.1 | The AWS account is a standalone account (not part of an organization, or is a linked account without org-wide cost access). | The cost overview shows only the single account's costs. Multi-account features are either hidden or show a single row. No error occurs. |
