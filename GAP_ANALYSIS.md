# Gap Analysis — Laravel/Filament → Go Fiber + SvelteKit

Date: 2026-11-14
Reference (read-only): `D:\Softcents\sms.whiteinfobd.com (2)`
Target: `E:\Softcents\sms.whiteinfobd.com (2)`

This document began as the Step 1 inventory + Step 2 gap list. It now also records the implemented fixes and the later 2026-11-15 audit corrections at the end. Database/browser verification is still outstanding (see caveats).

---

## 1. How this was produced

- Read the reference panels (`SupermanPanelProvider`, `CustomerPanelProvider`, `SpPanelProvider`), all navigation-group enums, all admin page classes, the admin/customer/SP resources, routes (`api.php`, `web.php`, `console.php`), middleware, jobs, console commands, services, settings pages and the relevant models/migrations.
- Read the target route tables (`superman-api`, `customer-api`, `sp-api`), the SvelteKit route tree, `superman-pages.ts`, the sidebars, and the external API implementation.
- Cross-checked every frontend API call against the registered Go routes with a script (normalising `:id` / `${...}`), and checked the important business rules by reading both implementations side by side.
- Baseline health check before analysis: `go build ./...` passes in all three services; `npm run check` → 0 errors, 127 warnings.

### Reference inventory (verified counts)

| Area | Count |
|---|---|
| Panels | 3 — Superman (`/superman`), Customer (`/`), SP (`/sp`) |
| Admin page classes | 143 (+ 21 resources, 43 widgets) |
| Admin navigation groups | 32 |
| Customer resources / pages | 14 / 12 |
| SP resources / pages | 7 / 2 (+ dashboard, KYC, queue, reports, wallet pages) |
| Migrations | 82 |
| Console commands | 5 (`PurgeStaleQueue`, `PollMessageDelivery`, `ExpireSubscriptions`, `SendDailyReports`, `ReconcilePayments`) |
| Jobs | 6 (`ProcessBatchJob`, `PollMessageDeliveryJob`, `ProcessCampaignJob`, `SendSmsJob`, `SendWebhookJob`, `ProcessDepositJob`) |
| External API | `/api/v1/{me,balance,senders,messages,dlr,webhooks,ping}` + public webhooks |

### Target inventory

| Area | State |
|---|---|
| Superman API | ~190 routes; admin nav 106 links / 19 groups |
| Customer API | ~90 routes; customer portal 40+ pages |
| SP API | 6 endpoints; SP portal 5 pages |
| External API | 8 endpoints (no DLR/webhook/ping surface) |
| Frontend | generic catch-all renders table pages from a static map; several pages are title-only shells |

---

## 2. P0 — production blockers (must fix before cut-over)

### P0-1. Customer registration is completely broken

- Frontend `frontend/src/routes/register/+page.svelte` posts **multipart/form-data** with `email_verification_token`, `phone_verification_token`, `nid_number`, `front_nid`, `back_nid`, `live_photo`, `trade_license_*`.
- Backend `customer-api/internal/httpapi/auth.go:163` calls `json.Unmarshal(context.Body(), ...)` and `registerRequest` has none of those fields → every registration gets **422**.
- Even ignoring the body format, the old flow (`app/Filament/Customer/Pages/Register.php`) is not implemented:
  - stores NID front/back + live photo + trade licence files on the private disk and records them on the user;
  - sets `status = pending`, `kyc_status = under_review`, `kyc_submitted_at = now()`, `trade_license_submitted`;
  - marks email/phone as verified from the OTP tokens;
  - rejects duplicate NID and restores soft-deleted accounts instead of creating duplicates;
  - attributes the new customer to an SP when a reseller signup link/plan is used.
- New code inserts `kyc_status = 'pending'`, stores no files, no NID uniqueness, no SP attribution.
- The OTP endpoints the signup form uses (`POST /otp/send`, `POST /otp/verify`, `customer-api/internal/httpapi/server.go:295`) are mounted inside the **authenticated** group, so both calls return **401** before any account exists. Signup can never obtain the email/phone verification tokens it requires. Old registration handles OTP in-page via Livewire with rate limits and 10-minute expiry.
- Login gating already matches old: `banned`, `rejected`, and `pending` + `kyc_status in (pending, under_review)` are blocked, with the `is_admin_created` exemption (`customer-api/internal/httpapi/auth.go:107-127` mirrors `Login::resolveAccountStatusMessage`). No change needed there.

**Fix scope:** accept multipart, persist KYC docs to the same storage layout, replicate status/timestamps, OTP verification, duplicate-NID/soft-delete handling, SP attribution; expose signup OTP publicly with rate limits. Login gating is already correct; portal gating is tracked as P0-3.

### P0-2. Developer API contract is incompatible with the old API

Old contract (`app/Http/Controllers/Api/V1/*`, `app/Support/ApiResponse.php`, `app/Enums/ApiResponseCode.php`):

```json
{"status":"success","code":1000,"message":"SMS sent successfully.","data":{...}}
```

- `POST /api/v1/messages` returns **HTTP 200** (or 202 for queued) with `message_id: "SMS{id}"`, `batch_id`, `recipients`, `status`, `code`.
- Accepts `phone`/`to`/`message` aliases as well as `sender_id`/`numbers`/`body`; accepts `schedule`/`scheduled_at`.
- `/me` and `/balance` wrap the payload in `data`.

New (`customer-api/internal/httpapi/external_api.go`): flat responses, HTTP 202, no `code`/`message_id`/`scheduled_at`, no `phone`/`to`/`message` aliases, no `schedule`, no `template_id` approval bypass. `POST /messages` sets approval correctly, but `GET /senders` returns `{data:[{sender_id,type}]}` with no envelope/status/code, and `GET /batches/:batch` returns a reduced payload (`total_recipients`, `valid_count`, `invalid_count`, `message_id` missing).
Enforcement is also missing: old `EnsureApiAccess` + controllers check `api_access`, account status, and `SecuritySettings` (`ip_whitelist_enforced`, `allow_all_when_whitelist_empty`, `auto_approve_api_requests`, `web_portal_bypass_enabled`, `trusted_portal_ips`, `blocked_message`) and write IP-audit rows. New `devAuthenticate` allows any token whose account exists (banned/pending included, `api_access` ignored), and treats “no approved IPs” as open access.

> This breaks every existing integration. Integrators will notice immediately.

### P0-3. Portal status/KYC gating is absent

- Old `HandleCustomerApproval` middleware: `pending` → pending page, `rejected` → rejected page, `banned` → banned page; those users may still open Dashboard, KYC, logout and email verification, and every other page redirects. Approved users are redirected away from the three status pages back to the dashboard. Admin-created pending accounts and impersonated sessions bypass the guard.
- New portal has only `/account/kyc` and `/account/pending`; **no rejected/banned pages**, no route guard, and `customer/+layout.svelte` renders the full portal for any logged-in status.
- Gaps that remain: a `pending` customer whose `kyc_status = resubmit_required` (login is allowed by the rule above) gets the full portal instead of the old KYC-resubmission page, and a banned/rejected customer with an existing JWT can use every portal page until it expires.

### P0-4. Three shipped screens call endpoints that do not exist

| Screen | Call | Result |
|---|---|---|
| Customer → Contacts → Edit | `PUT /api/v1/customer/contacts/:id` | 404; contact edits are silently lost |
| Customer → Groups → Edit | `GET /api/v1/customer/groups/:id/contacts` | 404; member list never loads |
| Admin → Security → IP Whitelist → Approve/Reject | `POST /api/v1/superman/approval-center/ip-whitelists/:id/:action` | 404; admin cannot approve IPs from that page |

### P0-5. Admin is shown fabricated operational data

The project rule “never use mock/fake data fallbacks” is violated in `superman-api/internal/httpapi/platform_ops.go`:

| Endpoint | Problem |
|---|---|
| `GET /system-settings/backups` | hardcoded backup list (`size_mb: 48.2 …`), hardcoded “last backup”; POST returns a fake snapshot |
| `GET/POST /system-settings/maintenance` | returns hardcoded `is_down:false`, secret `whiteinfo-admin-bypass`, notice and exempt IPs; POST does not persist and **maintenance mode is not enforced anywhere** (old toggled Laravel `down`/`up` for real) |
| `GET /system-settings/queue` | hardcoded “Redis (Horizon)”, 8 workers, `sms-worker-pool` — the Go stack has no Horizon; sizes are partly real |
| `GET /system-settings/error-codes` | hardcoded code table (old reads `ApiResponseCode` config; acceptable if config-driven, but currently not persisted) |
| `GET /security/dashboard` | real counts plus invented `blocked_attacks_24h: 14` and a static firewall label (old shows recent security logs instead; the new response omits them) |
| `GET /monitoring/tps` | `peak_today` is computed as `current_tps * 2` |
| `GET /sps/white-label` / `GET /sps/custom-domains` | parity with old (old also lists SP users and synthesises `sms.{id}.whiteinfobd.com`); the POST “saved” response has no persistence but old had no write path either |

- Backups are the largest missing feature: old `BackupSettingsPage` persists `BackupSettings` (auto-backup schedule, retention, FTP and Google Drive targets) and `DatabaseBackupService` can create, download and delete real dumps. The new page has no settings form and no real dump path.
- `System Settings → Notifications` is missing entirely (Telegram token/chat id, low-balance threshold, new-customer/new-deposit/gateway-error alerts, admin email/phone).

Operators rely on these screens during incidents; showing invented values is worse than showing “not available”.

### P0-6. SP portal is a shell (and its permissions model is missing)

- Target SP API has only: dashboard, customers list, fund wallet, wallet, billing, messages.
- Old SP panel has: customer create/edit/delete/export, tariff plans + per-operator rates, deposits, transactions/statements, employees CRUD, roles + granular `SpPermission` enforcement, senders, wallet distribution, KYC review/send-back, reports, live queue, password reset/profile/MFA.
- Old gates every SP resource/page through `SpPermissionService::can()` (`customers.view/create/edit/delete`, `billing.manage`, `employees.*`, `roles.*`, `messaging.*`, `reports.view`); the new SP API has no role or permission checks, so **any SP employee token can fund any of the SP's customers** — a direct money risk.

### P0-7. Customer/admin notifications are gone

Old uses Filament database notifications with 30-second polling: batch completed/failed, low balance, deposit approved/rejected, KYC result. Separate Telegram alerts fire for new customer, new deposit, gateway error and low provider balance during campaign runs, configured on `System Settings → Notifications`.
New: no `notifications` writes anywhere in the Go services; the customer navbar bell is a hardcoded “3”. The only Telegram alert implemented is the queue-wait warning (env-var driven, no admin settings page). Customers will not know a batch finished or failed.

---

## 3. Missing admin sections (59 pages across 14 absent navigation groups)

Present in the old sidebar, **absent** from `AdminSidebar.svelte` and from `superman-pages.ts` (no route, no endpoint). Most are real Filament tables, not decoration (page class checked in the reference).

| Group | Missing pages | Page type | Priority |
|---|---|---|---|
| Affiliates | Affiliates, Withdrawal Requests, Commission Management, Affiliate Reports, Affiliate Audit, Affiliate Dashboard | 4 tables + 2 info | P2 |
| SMS Management | All SMS, Bulk SMS, Pending, Failed, Scheduled, Retry Queue, Send SMS, SMS Inquiry, SMS Logs, Message Search | 7 tables + 3 forms | P1 |
| DLR | Live DLR, DLR Queue, DLR Retry, Delivery Logs, Delivery Check | 4 tables + 1 form | P1 |
| Bangladesh Network | Operators/prefixes, Number validation, MNP lookup, Coverage, Routing | mixed | P2 |
| Campaigns | Campaign List, Create Campaign, Campaign Reports | 2 tables + form | P2 (see §7) |
| Profit Management | Global Profit, Customer Profit, Reseller Profit, Profit Trend, Profit Alerts | 3 tables + 2 info | P2 |
| MO / Inbound | Inbound messages, Keywords, Auto-replies, Routing | mixed | P2 |
| Webhooks | Configuration, Customer Webhooks, Reseller Webhooks, Logs, Failed | 1 form + 4 tables | P1 |
| Alerts | Alert Rules, Alert History, Channels | mixed | P2 |
| Payment Gateway | Global Gateways, Transactions, Reconciliation, Payment Reports | 3 tables + 1 info | P1 |
| Employees | Employee List, Session Management | 2 tables | P1 |
| Roles & Permissions | Roles, Permissions | 2 matrix screens | P1 |
| API Management | API Accounts, API Keys, API Logs, API Security Policy | 3 tables + 1 form | P1 |
| API Documentation | Swagger/Scribe page | static | P2 |

Two more old pages are missing from groups that otherwise exist: **System Settings → Notifications** and **Resellers → Payment Gateway** (see §4).

---

## 4. Admin sections with missing or partial backend support

| Page | Gap |
|---|---|
| Settings → Countries | `GET /countries` only; old has full CRUD + dial code/prefix validation |
| Settings → Operators | `GET /operators` only; old has CRUD, logo upload, prefix mapping |
| Settings → Email Gateways | **No page and no endpoint** — sidebar link renders an empty shell |
| Settings → Signup Register Gateways | **No page and no endpoint** — sidebar link renders an empty shell |
| System Logs → Email Logs | No `email_logs` table writes anywhere in Go; old logs every `MessageSent` |
| System Settings → Notifications | **Page missing entirely**; old persists `NotificationSettings` (Telegram token/chat id, low-balance threshold, alert-on-campaign, new-customer, new-deposit, gateway-error, admin email/phone) |
| Resellers → Payment Gateway | **Page missing** (`ResellerPaymentGatewayPage` exists in old nav); no endpoint in Go |
| Messaging → Approvals | Approve/reject actions not backed by Go routes; bulk send never sets `approval` (see §6) |
| Security → IP Whitelist | Request actions point at a non-existent URL (P0-4) |
| System Settings → Backups/Maintenance/Queue | fabricated data (P0-5) |

---

## 5. Customer portal gaps

### Pages
| Old page | New | Status |
|---|---|---|
| Register (KYC upload + OTP) | `/register` | broken (P0-1) |
| Account → Rejected | — | missing |
| Account → Banned | — | missing |
| Account → Pending | `/account/pending` | present, not enforced |
| Messages → View message | — | missing (list only) |
| Templates → Edit/View | `/customer/templates` | create/delete only |
| Contacts → Edit | `/customer/contacts` | API 404 |
| Groups → Members | `/customer/groups/[id]/edit` | API 404 |
| Billing analytics | `/customer/analytics/billing` | present |
| Live queue | `/customer/live-queue` | present |

### Behaviour
- No status/KYC route guard (P0-3).
- Profile: old allows name/phone/email change with email re-verification; new `updateProfile` handles name/phone only (`auth.go:369`), and no email-change flow exists.
- Password change/reset present, but no “current session”/token revoke list parity.
- 2FA (TOTP) is required by the old panel (`AppAuthentication`); the new portal has no MFA at all.
- No notifications (P0-7).
- Sender request: old allows selecting gateway and shows approval history; new form is minimal.
- Quick send: old supports sender auto-selection, unicode detection, balance preview and per-recipient validation; new supports most of it but not approval gating.

---

## 6. Business-rule differences that change money or message state

| # | Rule | Old | New | Risk |
|---|---|---|---|---|
| B1 | Manual SMS approval (portal) | `requiresSmsApproval()` — when `sms_auto_approved = 0` every portal batch is created with `approval = pending` and **not sent** until an admin approves; approved templates bypass | external API enforces it, but the **portal** `bulkSend`/quick send never checks `sms_auto_approved` and never sets `approval` | Customers who must be moderated can send freely from the portal |
| B2 | Per-row CLI in bulk CSV | CSV may carry a `cli`/`sender` column; each row resolves its own sender (and `bill_msisdn` for IPTSP) | `ProcessBatchJob` equivalent uses the batch-level `sender_id` for every row; no `bill_msisdn` | Bulk uploads produce wrong sender/billing |
| B3 | Stale queue purge | after 8 min, messages marked failed **without refund** (`PurgeStaleQueue`) | scheduler refunds the customer | Over-refunds; revenue leak |
| B4 | Queue purge scope | statuses `pending`/`processing`, based on `updated_at` | same query, plus refund | see B3 |
| B5 | SP wallet top-up | creates a deposit + transaction ledger entry with before/after balances | `UPDATE users SET balance = balance + ?` only (`platform_ops.go:351`) | No audit trail for reseller money |
| B6 | API access | `api_access` must be enabled; blocked otherwise | not checked | Disabled customers can use the API |
| B7 | Banned/pending API use | rejected with `ACCOUNT_SUSPENDED` (2003) and `blocked_message` | accepted | See P0-2 |
| B8 | IP whitelist policy | four `SecuritySettings` toggles + audit log | “no approved IPs = open” only | Security policy silently ignored |
| B9 | Scheduled API sends | accepted and queued for later | ignored | Integrators’ scheduled messages send immediately |
| B10 | Batch cancel/refund | only unsent messages refunded, ledger entry written | verify per-status refund set (needs test) | Money |

---

## 7. Background processing / system behaviour

| Item | Old | New | Status |
|---|---|---|---|
| Batch dispatch | `ProcessBatchJob` chunks, per-row sender, ledger debit, failed CSV | present (`batch/processor.go`), debit + failed CSV present | partial (B2) |
| DLR poll | 5 attempts, 60/120/120/120 s, 5-min fallback sent→delivered, 48 h max | matches per `PRODUCTION_GAPS_REPORT.md`; **re-verify in code during implementation** | likely OK |
| Stale queue purge | 8 min, messages **and batches** marked failed, no refund | scheduler marks stale messages failed **and refunds**; verify batch handling | different (B3) |
| Campaigns | scheduler flips status only; resource hidden from nav | not implemented | intentionally dormant |
| Subscription expiry | command exists but is **not scheduled** in `routes/console.php` | not implemented | no behaviour to port |
| Daily reports | command exists but is **not scheduled** | not implemented | no behaviour to port |
| Payment reconciliation | command exists but is **not scheduled** | `cmd/reconcile` exists | no behaviour to port |
| Webhooks (outgoing) | `SendWebhookJob`, retries, logs, failed list | `webhooks` table exists? customer API has webhook receive; no outgoing dispatcher | missing |
| Notifications | DB notifications + 30 s poll | none | missing (P0-7) |
| Telegram queue alert | sent by Horizon/queue monitoring | `runTelegramAlertCheck` every minute (>60 s queue wait) | present |
| Maintenance mode | real `artisan down/up` | flag not persisted/enforced | missing (P0-5) |
| Backups | `DatabaseBackupService` (real dumps + retention) | fake list | missing (P0-5) |
| Audit logs | login, IP, employee, security, model events | partial endpoints only | partial |

---

## 8. API surface diff (developer API)

The **route surface is identical** (`/me`, `/balance`, `/senders`, `GET|POST /messages`, `/batches/:batch`, `/ip-whitelists`). The differences are in response shape and enforcement:

| Endpoint | Old | New |
|---|---|---|
| `POST /api/v1/messages` | envelope, HTTP 200, `message_id: SMS{id}`, `batch_id`, `status`, `approval`, `recipients`, `scheduled_at`; `phone`/`to`/`message` aliases; `schedule`/`scheduled_at`; `template_id` bypass; `api_access`/status/IP checks; 402 error envelope | flat, HTTP 202, no envelope/`message_id`/schedule/aliases; approval enforced; no `api_access`/status checks |
| `GET /api/v1/me` | `{status,code,message,data}`, `data` = name/email/phone/balance/account type | flat `{id,name,email,phone,balance,approval}` |
| `GET /api/v1/balance` | envelope with balance/currency | flat `{balance,currency}` |
| `GET /api/v1/senders` | envelope, all assigned senders incl. pending/rejected + status | `{data}`, enabled only, no status |
| `GET /api/v1/batches/:batch` | envelope 1003, `message_id`, totals, valid/invalid, timestamps | flat, reduced fields, 404 message |
| IP whitelists | envelope + `store` validation (`ip_address` format, duplicate pending check) | `{data}` list, create/delete; validation to be compared |
| IP enforcement | `SecuritySettings` toggles + audit rows | “no approved IPs = open” |

---

## 9. UI / navigation differences

- Admin: 19 groups vs 32; missing groups listed in §3. Existing groups are in a different order, and several labels differ (`User` → “Users”, `SenderCli` → “Sender IDs”, `Providers` → “SMS Gateways”). Old users navigate by group name; order and wording should be restored.
- Old admin global search, per-page column toggles, bulk actions, CSV/Excel export, and “create” modals are not reproduced by the generic table renderer. Pages that rely on them (Customers, Senders, Tariff Plans, Deposits) are visually similar but functionally thinner.
- Customer: sidebar contains extra items that old hides until KYC approval; notification bell static; mobile layout differs (old uses Filament’s topbar/sidebar behaviour).
- SP: 5 links vs 13; no permission-based menu filtering.
- Filament look: gray-50 background, white cards, 1 px borders, 8 px radius, emerald primary, 13 px table text, inline filters above tables, right-side slide-over forms. The new Tailwind pages approximate this but not consistently (some pages use 11 px labels, different paddings, different empty states).

---

## 10. Proposed implementation order

### Phase 0 — correctness & security (do first, ship-blocking)
1. **P0-1 Registration + login status** (customer-api auth, frontend register, storage).
2. **P0-3 Portal gating + rejected/banned pages** (Svelte layout guard + two pages).
3. **P0-2 External API compatibility layer** (envelope, codes, aliases, schedule, status/api_access/IP policy, `/dlr`, `/ping`, rate limit).
4. **P0-4 Broken routes** (contact PUT, group contacts GET, IP-whitelist action URL or matching route).
5. **P0-5 Remove fabricated admin data**: real backup settings + dumps, persisted and enforced maintenance mode, queue stats from the Go worker pool, and remove invented security/TPS figures and Horizon references.
6. **P0-7 Notifications**: DB `notifications` table read/write, batch/deposit/KYC events, navbar polling.
7. **B1 approval gating + admin approve/reject routes**.

### Phase 1 — customer portal parity
Templates edit/view, message detail, profile/email change, sender request parity, dashboard widgets (balance/usage/trend/recent), export formats, UI polish to Filament look.

### Phase 2 — admin parity
- SMS Management (10), DLR (5), Webhooks (5), Payment Gateway (4), API Management (4).
- Employees, Roles/Permissions (matrix + enforcement on superman-api).
- Countries/Operators CRUD, Email/Signup gateways, Email logs.
- Affiliates, Profit Management, Alerts, BD Network, MO/Inbound (confirm which are informational first).
- Admin global search, column toggles, bulk actions, exports.

### Phase 3 — SP parity
Customers CRUD/export, tariff plans + rates, deposits/statements, employees + roles with `SpPermission` enforcement (fixes money risk), senders, wallet distribution, KYC review/send-back, reports, live queue, profile/password/MFA.

### Phase 4 — system behaviour
Campaigns (only if you want them enabled), subscription expiry, daily reports, payment reconciliation, outgoing webhooks, 2FA for all panels, DLR/purge rule decisions (B2/B3/B9).

Each phase item will be implemented backend + frontend, then checked with `go build`, `npm run check`, and focused endpoint tests.

---

## 11. Questions I need answered before Phase 0

1. **API compatibility:** confirm the old envelope/codes/aliases/scheduling must be preserved exactly (recommended: yes, existing integrations depend on it).
2. **Purge refund (B3):** keep the old “no refund” rule, or keep the new refund behaviour? Money semantics must be one or the other.
3. **Bulk per-row CLI / bill_msisdn (B2):** is this used by live customers? It changes the batch processor significantly.
4. **Maintenance mode scope:** block customer portal + developer API only, or also SP and admin? Old blocked the whole Laravel app except the secret bypass.
5. **Backups:** implement real `mysqldump`-style backups in Go (needs `mysqldump` on the host), or replace the page with an honest “managed by infrastructure” panel? Do not want fabricated data either way.
6. **Campaigns:** old hid the customer resource and the scheduler never sent. Confirm we leave campaigns out (recommended) or fully build them.
7. **2FA:** required for all three panels (old behaviour), or customer portal only?
8. **Informational pages:** MO/Inbound, BD Network coverage, API docs — reproduce as static reference pages (recommended), or skip them?
9. **Affiliates:** the old pages read SP users (`sp_id`) and commissions. Is the affiliate programme actually in use? If not, P2.
10. **Look & feel:** should I port Filament’s exact table chrome (column toggles, bulk bar, slide-over forms) or keep the current Tailwind components restyled to match? Recommended: restyle current components, add missing behaviours.

---

## 12. What is already at parity (spot-checked, no action)

- Password hashing (`bcrypt` cost 12), token format `id|token` with SHA-256 at rest.
- Batch debit ledger with before/after balance and failed-recipient CSV.
- External API route surface (same 8 endpoints as `routes/api.php`).
- DLR poller started with the worker, retry schedule and 5-minute fallback (`internal/dlr/poller.go`), matching `PollMessageDeliveryJob`.
- Tariff plan/rate resolution, operator detection, unicode part counting.
- Deposit approval flow, transactions list, IP whitelist table, KYC review endpoints.
- Sender CRUD, gateway CRUD + encrypted credentials, routing rules, audit log endpoints.
- Most customer list/analytics/dashboard pages exist and hit live endpoints.

## Appendix A — Old admin page inventory (143 pages, by navigation group)

Bold groups are **absent from the new admin sidebar** (59 pages). Together with System Settings → Notifications and Resellers → Payment Gateway, **61 old admin pages are missing**.

- ApprovalCenter (4): ApprovalCenterPage, PendingCustomerApprovalPage, PendingResellerApprovalPage, PendingSenderApprovalPage
- **Affiliates (6)**: AffiliateAuditPage, AffiliateDashboardPage, AffiliateReportsPage, AffiliatesPage, CommissionManagementPage, WithdrawalRequestsPage
- **Alerts (3)**: AlertConfigurationPage, AlertDashboardPage, ProviderDownPage
- **ApiDocumentation (1)**: ApiDocumentationPage
- **ApiManagement (4)**: ApiAccountsPage, ApiKeysPage, ApiLogsMgmtPage, ApiSecurityPage
- AuditLogs (4): EmployeeActivityPage, GlobalAuditLogPage, IpWhitelistAuditPage, SecurityActivityPage
- **BangladeshNetwork (5)**: DndPage, MnoManagementPage, MnpPage, NumberValidationPage, PrefixManagementPage
- Billing (6): BillingReportsPage, InvoicesPage, PaymentsPage, RateHistoryPage, UsageCdrPage, UserAssignmentPage
- **Campaigns (3)**: CampaignListPage, CampaignReportsPage, CreateCampaignPage
- **Dlr (5)**: DeliveryCheckPage, DeliveryLogsPage, DlrQueuePage, DlrRetryPage, LiveDlrPage
- **Employees (2)**: EmployeeListPage, SessionManagementPage
- Kyc (4): KycAuditPage, KycDashboardPage, KycPermissionsPage, VerificationRequestsPage
- **MoInbound (4)**: MoLogsPage, MoMessagesPage, MoReportsPage, MoRoutingPage
- Monitoring (6): ApiHealthPage, LiveMonitoringPage, ProviderHealthMonitorPage, QueueHealthPage, SystemHealthPage, TpsMonitorPage
- **PaymentGateway (4)**: GlobalPaymentGatewaysPage, PaymentReportsPage, PaymentTransactionsPage, ReconciliationPage
- **ProfitManagement (5)**: CustomerProfitPage, GlobalProfitPage, ProfitAlertsPage, ProfitTrendPage, ResellerProfitMgmtPage
- Providers (4): DlrMappingPage, ErrorMappingPage, ProviderApiLogsPage, ProviderHealthPage
- Reports (8 + base): CustomerReportsPage, ExecutiveReportPage, FinancialReportsPage, MaskingReportPage, NonMaskingReportPage, ProviderReportsPage, ResellerReportsMgmtPage, SmsReportsPage
- Resellers (18): CustomDomainPage, ResellerApiPage, ResellerAuditPage, ResellerBillingPage, ResellerCustomersPage, ResellerEmployeesPage, ResellerKycPage, **ResellerPaymentGatewayPage**, ResellerProfitPage, ResellerRatesPage, ResellerReportsPage, ResellerRoutingPage, ResellerSenderPage, ResellerStatementsPage, ResellerWalletPage, ResellerWebhookPage, ResellersPage, WhiteLabelPage
- **RolesPermissions (2)**: PermissionsPage, RolesPage
- Routing (7): FailoverPage, LoadBalancingPage, RouteHealthPage, RouteListPage, RouteTestingPage, RoutingDashboardPage, RoutingRulesPage
- Security (4): FraudRulesPage, IpWhitelistPage, SecurityDashboardPage, SecurityLogsPage
- SenderCli (5): MaskingListPage, NonMaskingListPage, SenderAuditPage, SenderPolicyPage, SenderResolutionPage
- **SmsManagement (10)**: AllSmsPage, BulkSmsPage, FailedSmsPage, MessageSearchPage, PendingSmsPage, RetryQueuePage, ScheduledSmsPage, SendSmsPage, SmsInquiryPage, SmsLogsPage
- SystemLogs (4 + EmailLog resource): ErrorLogsPage, SystemApiLogsPage, SystemLogsPage, SystemSmsLogsPage
- SystemSettings (9): BackupSettingsPage, DlrSettingsPage, ErrorCodeManagementPage, MaintenanceModePage, **NotificationSettingsPage**, ProviderSettingsPage, QueueSettingsPage, RoutingSettingsPage, SmsSettingsPage
- **Webhooks (5)**: CustomerWebhooksPage, FailedWebhooksPage, ResellerWebhooksPage, WebhookConfigurationPage, WebhookLogsPage

Resource-based groups (no page class): User (Customers, SPs), Audience (Contacts, Groups), Messaging (Messages, Batches, Templates, Approvals), Billing (Tariff Plans, Deposits, Transactions), Settings (Countries, Operators, Email Gateways, Signup Register Gateways), Providers (Gateways), SenderCli (Senders), Roles/Permissions (RoleResource), ApprovalCenter (IpWhitelistRequestResource), SystemLogs (EmailLogResource).

## Appendix B — Old customer / SP / background inventory

Customer resources (14): AccessToken, Batch, Campaign *(hidden)*, Contact, Deposit, Export *(hidden)*, Group, Import *(hidden)*, IpWhitelist, Message, ScheduledMessage, Sender, Template, Transaction.
Customer pages (12): Dashboard, BillingAnalytics, MessagingAnalytics, LiveQueuePage, Pricing, Login, Register, Account/{Page,Kyc,Pending,Rejected,Banned}.

SP resources (7): Customer, Deposit, Employee, Role, Sender, TariffPlan, Transaction.
SP pages: Dashboard, KycVerificationPage, LiveQueuePage, Reports, Wallet (plus profile/password/MFA).

Console commands: `messages:poll-delivery`, `queue:purge-stale`, `batches:send-scheduled`, `campaigns:send-scheduled` (every minute); `telescope:prune`, `disposable:update` (daily).
Jobs: `ProcessBatchJob`, `PollMessageDeliveryJob`, `ProcessCampaignJob`, `SendSmsJob`, `SendWebhookJob`, `ProcessDepositJob`.

## Phase 0 implementation log (2026-11-14)

- **P0-1 Registration/OTP/KYC**: multipart + JSON registration, public signup OTP (`/otp/send`, `/otp/verify`), old duplicate/soft-delete rules, private KYC paths, pending/under-review state, plan/slug assignment, reseller plan validation. Signup OTP now tries `signup_register_gateways` first, then OTP gateways.
- **P0-2 Portal gating**: pending/rejected/banned status pages; dashboard + KYC remain reachable for restricted accounts (matches `HandleCustomerApproval`); first KYC is locked server-side, SP-created resubmissions move to `awaiting_sp_verification`, regular submissions to `under_review`.
- **P0-3 Developer API**: restored `{status, code, message}` envelope, API code table, `phone`/`to`/`message` aliases, GET/POST sends, schedule/scheduled_at, `template_id` approval bypass, `{id}|{token}` auth, `api_access`/status checks, IP whitelist policy with portal bypass and blocked-message support, batch/DLR fields and whitelist API.
- **P0-4 Broken routes**: contact update, group member listing, admin IP approval route, real backup download.
- **P0-5 Fabricated data**: real backup files from `mysqldump`, persisted maintenance mode (enforced in customer/SP APIs), real queue/capacity/peak-TPS values, real security counts, API code table matches the old enum.
- **P0-6 Notifications**: Laravel `notifications` table is read/written; customer and admin bells poll every 30s, list/read/read-all endpoints, batch completion/failure, deposits, KYC results/resubmission events.
- **P0-7 Approval gating**: portal quick/bulk/scheduled batches now honor `sms_auto_approved` and wait for admin approval.
- **SP permissions**: employee tokens now inherit the owner's data scope and are checked against `sp_roles.permissions` (`customers.view`, `billing.view`, `billing.manage`, `messaging.view`); owners retain full access.

Assumptions recorded: maintenance is enforced on customer + SP APIs but the admin API remains reachable for recovery; backup storage is local (`BACKUP_PATH`, default `storage/backups`) with FTPS/Drive targets still outstanding; admin notifications only appear for events that write them (no synthetic entries).
- **P0-6 SP portal**: added customer create/edit/delete/restore/export + plan assignment and negative (reversal) balance distributions with the old customer-balance/SP-balance checks; SP-first KYC review (verify → `under_review`, send back → `resubmit_required` with requested fields and customer notification); employees CRUD; roles CRUD with the full old permission list enforced server-side; sender requests; tariff plans with per-operator masking/non-masking rates and archive; deposits; live queue; reports (traffic, customer spend, distributions, top customers); profile and password. SP owners keep full access; employees inherit the owner's data scope and are limited to their role permissions. Admin notifications are now written for new KYC submissions and SP verification.
- Known SP gaps still open: app-based MFA and self-service password reset for the SP login, and provider-cost/margin reporting is shown as customer spend + distributions instead of estimating wholesale cost.
- **Stale-queue purge (Decision 2)**: `customer-api/internal/scheduler/schedulers.go` (`runPurgeStale`) enforces the legacy `PurgeStaleQueue` rule (mark stuck messages and batches `failed` after 8 minutes with **no refund**), removing the conflicting refund path. Note for future review if automated refunds on confirmed unsent timeouts are ever desired.

## Customer parity continuation log (2026-11-15)

- **2FA compatibility:** Filament AppAuthentication stores `app_authentication_secret` with Laravel's `encrypted` cast and recovery codes with `encrypted:array` after bcrypt-hashing each code. The Go portal now reads those values with `APP_KEY` and writes the same format; recovery codes remain single-use. Existing plaintext Go-era values are still accepted.
- **MFA column type:** the reference base migration declares JSON for recovery codes but the encrypted cast writes a string; the parity migration normalises the column to LONGTEXT. Apply on staging and verify before production.
- **Email change:** implemented as password-confirmed, one-hour, single-use token in `email_change_requests`, matching Filament's `emailChangeVerification` intent (the old package stores the pending change in its signed link; the Go implementation stores only a SHA-256 token hash).
- **Developer API docs:** the old customer nav links to a separately generated Scribe `/docs` site. The Go frontend now serves an equivalent endpoint reference at `/docs`, gated by `api_access` in the sidebar.
- **Messages `API User`:** not fabricated. The reference resolves it from gateway request payload `username`; the new schema does not persist that field.
- **Dashboard card set:** matches the reference 9 cards; the earlier extra “Successful SMS” card was removed and Groups added.
- **Top Up:** the dashboard button opens the existing deposit modal through `?deposit=1` instead of stopping on the deposit list page.
- **PDF footer:** changed to the reference `Generated by Sendora SMS Platform`.
- **Unverified:** all database-backed flows remain unverified on this workstation because no MySQL/MariaDB or `mysqldump` is available; see `CUSTOMER_PORTAL_PARITY.md` → Final verification log.

## Second customer-portal audit (2026-11-15)

Direct re-reading of the customer Filament pages uncovered and fixed additional visible gaps:

- Message list now searches numeric message IDs, groups by status (collapsed by default), exposes the reference per-page control, shows all 19 columns (API User as an explicit placeholder where no request payload exists), formats Submit/Delivery as time-only, displays BDT currency, and renders missing previous/next balances as `—` rather than `0.00`.
- The message detail modal now contains the reference fields and hides Sent At/Failed Reason when absent; the batch log opens this shared detail instead of a reduced bespoke modal.
- Sent Message Logs now uses `failed_at` in its date fallback, exposes exactly the six legacy statuses, and treats missing balances as absent.
- Dashboard restores the default Sent At column, opens message detail from recent rows, abbreviates contact/group/template totals, and no longer adds an unrequested failed-only filter to the Failed Today link.
- Live Queue now uses `pending`/`sending` for In Queue Now and only `processing` for Active Batches, matching the reference widget and monitor.
- Deposits uses the reference `Top Up` header label and status labels; Transactions uses `Message Batch`/`S P Distribution`; transaction totals/details remain the reference set.
- DataTable gained reference-style grouping and per-page selection without changing unrelated pages.

Known unresolved parity risks are recorded in `CUSTOMER_PORTAL_PARITY.md` → “Post-commit audit corrections”: real `API User` payload data is not stored by the schema; no MySQL/browser walkthrough was possible; modal formatting is not server-side Filament-identical.
