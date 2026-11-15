# Customer Portal Parity Checklist

**Source of Truth (Reference — READ-ONLY)**: `D:\Softcents\sms.whiteinfobd.com (2)` (Laravel + Filament v4/v5 Customer Panel)  
**Target**: `e:\Softcents\sms.whiteinfobd.com (2)` (`customer-api/` Go Fiber v3 + MySQL `sms_panel` + `frontend/` SvelteKit 2 / Svelte 5 Runes / Tailwind v4)  
**Locked Decisions**:
- **API compatibility**: Preserve legacy Developer API envelope (`{status, code, message}` + endpoint fields), HTTP status codes, aliases (`phone`/`to`/`message`), scheduling (`schedule`/`scheduled_at`), `template_id` bypass, `api_access`, account status, and IP whitelist policy (`SecuritySettings`) + audit logs.
- **Stale-queue purge**: Legacy rule (mark stuck messages/batches `failed` after 8 min, **no refund**).
- **Campaigns**: Left out (hidden and never dispatched in Reference).
- **Honest states**: No mock/fake data or placeholder fallbacks.

Status Legend: `NOT STARTED` | `IN PROGRESS` | `IMPLEMENTED` | `VERIFIED`

---

## Priority 0 — Database Schema & Core Query Compatibility

### 0.1 Schema & Migration Alignment (`sms_panel` DB vs Reference Migrations 2026-10-21..25)
- **Reference Behavior**:
  - `tariff_plan_assignments` has column `status` (default `'active'`), **not** `is_active`. `OperatorRateResolver` requires `tpa.status = 'active'`, `tpa.effective_from <= CURDATE()`, `tp.status = 'active'`, cascading: Active TariffPlan (`tariff_rates` latest `effective_from <= today`) -> `user_operator_rates` -> `operators` default.
  - `users` has `created_by_admin` (`tinyint(1)`), **not** `is_admin_created`.
  - Migration `2026_10_21_000001_create_email_logs_table.php` creates `email_logs`.
  - Migration `2026_10_25_120500_add_approval_columns_to_templates_table.php` adds `templates.approval` (`default 'approved'`), `templates.approved_by`, `templates.approved_at`, `templates.rejection_reason`, and `batches.template_id`.
  - Migration `2026_10_25_140000_add_sp_kyc_verification_to_users_table.php` adds `users.created_by_sp`, `users.kyc_sp_verified_by`, `users.kyc_sp_verified_at`.
- **Target Gaps**:
  - Local `sms_panel` dump stops at `2026_10_07_095620_add_deposited_by_id_to_deposits_table` (missing `2026_10_21`/`2026_10_25` columns/tables, causing runtime SQL errors when `customer-api` queries `templates.approval`, `batches.template_id`, `users.created_by_sp`, `users.kyc_sp_verified_by`, `users.kyc_sp_verified_at`).
  - `customer-api/internal/billing/operator_rate_resolver.go:100` & `resources.go:1574`: query non-existent `tpa.is_active = 1`.
  - `customer-api/internal/httpapi/resources.go` `pricing()` duplicates rate logic and omits `effective_from <= CURDATE()`, `tp.status = 'active'`, and `user_operator_rates` fallback.
  - `customer-api/internal/httpapi/auth.go:130`: queries non-existent `is_admin_created` instead of `created_by_admin`.
- **Status**: `NOT STARTED`
- **How Verified**: Pending SQL migration script / `DESCRIBE` verification + live `customer-api` billing/pricing/login/template tests against MySQL.

---

## Priority 1 — Critical Auth, Account Status, KYC, Money, Security & Messaging Blockers

### 1.1 Login, Password Reset, Profile, Email Change & 2FA (TOTP)
- **Reference Behavior & UI**:
  - `Login.php`: email/phone + password login; checks `UserStatus`: Approved allowed; Pending -> `account.pending`; Rejected -> `account.rejected`; Banned -> explicit error `"Your account has been banned due to a violation of our Terms & Conditions. Please contact the administrator for assistance."` & redirect/block; enforces `AppAuthentication` (TOTP 6-digit code or 8 recovery codes when `app_authentication_secret` is set). Updates `last_login_at`.
  - `CustomerPanelProvider`: `->passwordReset()`, `->profile()`, `->emailChangeVerification()`, `->multiFactorAuthentication([AppAuthentication::make()->recoverable()])`.
  - Profile page: update name/phone/password, **change email with verification flow**, enable/disable Authenticator App 2FA (QR secret + verify code) and view/regenerate recovery codes (`app_authentication_secret`, `app_authentication_recovery_codes`).
  - `/terms` route (`routes/web.php:67`): returns terms page (`'Comming soon...'` / legal notice).
- **Target Gaps**:
  - `auth.go:130` queries non-existent `is_admin_created`.
  - `login/+page.svelte` & `auth.go`: verify full banned/rejected/pending status handling + 2FA challenge flow (`app_authentication_secret` / recovery codes).
  - `customer/profile/+page.svelte`: email input is disabled (missing email change verification flow) and missing 2FA (`AppAuthentication` QR setup/disable + recovery codes) section.
  - `/terms` SvelteKit route missing (linked in `register/+page.svelte`).
- **Status**: `NOT STARTED`
- **How Verified**: Pending unit + live API + browser auth/2FA/profile/reset tests.

### 1.2 Registration, OTP Verification, Referral Slugs & Phone QR Capture
- **Reference Behavior & UI**:
  - `Register.php` + `SignupLinkController`: public `/register` and `/register/{slug}` (`tariff_plans.slug` or `signup_slug_aliases` -> auto-assign `tariff_plan_id` in `tariff_plan_assignments` + `sp_id`, `status = 'active'`, `effective_from = today`).
  - Inline Email OTP (`verification_otps` `type='email'`, cooldown & hourly cap, `email_verified_at` stamped).
  - Inline Phone OTP (`verification_otps` `type='phone'`, dispatched via active `signup_register_gateways`, `phone_verified_at` stamped).
  - Live Photo Capture (`live-photo-capture.blade.php` + `PhonePhotoController` + `phone_photo_sessions`): on desktop, **"Take Photo via Phone (QR)"** creates `phone_photo_sessions` token, renders real scannable QR code pointing to `/phone-photo/{token}`, shows expiry countdown, polls session status until `completed` + photo preview populates. `/phone-photo/{token}` renders mobile camera UI (`already-completed.blade.php` if already uploaded, error if expired/invalid, camera capture + `Retake` + `Use This Photo` POSTing photo data).
  - Front NID, Back NID, NID Number (`unique` check), Business Name, Business Address, Optional Trade License (Number + Image/PDF), Password, Terms checkbox (`I agree to the Terms & Conditions and Privacy Policy` -> `/terms`).
  - Sets `status = 'pending'`, `kyc_status = 'under_review'`, `kyc_submitted_at = now()`, `nid_verified = false`, `live_photo_verified = false`, `trade_license_submitted = !empty(trade_license)`.
- **Target Gaps**:
  - `register/+page.svelte:1239-1241`: Phone QR modal shows static fake text `[QR Code Available On HTTPS / Production Host]` instead of rendering an actual scannable QR code + polling for uploaded photo.
  - `frontend/src/routes/phone-photo/[token]/+page.svelte` route is completely missing!
  - `frontend/src/routes/terms/+page.svelte` is missing.
  - Verify `register.go` assigns `tariff_plan_assignments.status = 'active'` (compatible with DB schema) and validates uniqueness, OTP tokens, and referral slug cleanly.
- **Status**: `NOT STARTED`
- **How Verified**: Pending live registration + QR session + `/phone-photo/[token]` upload + OTP + DB verification.

### 1.3 Account Status Gating & KYC Resubmission (`pending`, `under_review`, `rejected`, `banned`, `resubmit_required`)
- **Reference Behavior & UI**:
  - `HandleCustomerApproval` + `HandleCustomerKyc` + `Account/{Pending,Rejected,Banned,Kyc}.php`:
    - `banned` -> locked on `/account/banned` (`"Your account has been banned due to violations of our Terms & Conditions..."`).
    - `rejected` -> locked on `/account/rejected`.
    - `pending` with `kyc_status === 'resubmit_required'` OR `kyc_status === 'pending'` (SP-created customers) -> `/account/kyc` (also accessible via sidebar `KYC Documents` even for approved customers).
    - `pending` with `kyc_status === 'under_review'` -> `/account/pending`.
    - Note on Trade License & KYC files: accepts images AND PDF (`.pdf`, `application/pdf`) for `trade_license_file`; only submits fields listed in `kyc_resubmit_fields` when resubmitting (or full initial KYC if SP-created/pending); resets `kyc_status = 'under_review'`, clears `kyc_sp_verified_by` & `kyc_sp_verified_at` when SP customer resubmits, logs `kyc_audit_logs`, notifies SP/Admin.
- **Target Gaps**:
  - `customer-api/internal/httpapi/resources.go:1354-1402`: `kycSubmit` passes `allowPDF = false` to `saveUploadedFile` for `trade_license_file` (blocks PDF trade licenses!).
  - `frontend/src/routes/customer/kyc/+page.svelte:455`: `<input type="file" accept="image/*" ...>` omits `.pdf` on Trade License.
  - `kyc/+page.svelte:170`: always appends `nid_number` even if not requested in `kyc_resubmit_fields`; lacks Phone QR live-photo option on KYC page; check redirect gating in `account/+layout.svelte` / `customer/+layout.svelte` for all status combinations (`pending` vs `resubmit_required` vs `rejected` vs `banned` vs `approved`).
- **Status**: `NOT STARTED`
- **How Verified**: Pending KYC upload tests (image + PDF + `resubmit_required` + `created_by_sp` flow + gating across statuses).

### 1.4 Developer External API Contract (`/api/v1/*`)
- **Reference Behavior & UI**:
  - Auth: Sanctum Bearer `<access_tokens.id>|<plain_token>` (`ProcessApiKey`) OR query/form/JSON `api_key`; checks `api_access == true`, user `status == 'approved'`.
  - Envelope: `{status, code, message, ...}` matching `ApiResponseCode` enum & HTTP status codes (`200`, `201`, `400`, `401`, `402`, `403`, `404`, `422`, `429`, `500`, `503`).
  - IP Whitelist policy (`SecuritySettings`: `ip_whitelist_enforced`, `allow_all_when_whitelist_empty`, `blocked_message`, `web_portal_bypass_enabled`, `trusted_portal_ips`; checks CIDR + exact IP against approved `ip_whitelist_requests`; writes `ip_whitelist_audit_logs` on create/edit/delete).
  - Endpoints:
    - `GET/POST /api/v1/sms/send`, `POST /api/v1/sms/send/batch`, `POST /api/v1/sms/bulk`: field aliases (`phone`/`to`/`mobile`, `message`/`body`/`text`), `sender_id` fallback ONLY when intentionally omitted (must NOT swallow explicit invalid/unauthorized `sender_id` in JSON vs form), scheduling (`schedule`/`scheduled_at`), `template_id` approval bypass, `sms_auto_approved` rule (1 recipient Quick vs Bulk), affordability check (skipped or handled properly on approval-required? Verify Reference `Api/V1/SmsController` behavior!).
    - `GET /api/v1/sms/status/{id}`, `GET /api/v1/balance`, `GET /api/v1/senders`, `GET /api/v1/templates`, `GET /api/v1/batches`, `GET /api/v1/batches/{id}`, `GET /api/v1/messages`, `GET/POST/DELETE /api/v1/ip-whitelist`.
- **Target Gaps**:
  - `external_api.go:530-590`: JSON fallback bug when `sender_id` is omitted or explicitly sent in JSON/form (`sender_id` parsing).
  - Verify `external_api.go` against Reference `Api/V1/*Controllers` + `EnsureApiAccess` + `ProcessApiKey` for all endpoints, HTTP codes, error codes (`ApiResponseCode`), `ip_whitelist_audit_logs`, and `template_id` bypass.
- **Status**: `NOT STARTED`
- **How Verified**: Pending Go HTTP tests / live `curl` matrix covering valid/invalid API keys, `api_access=0`, IP blocked/allowed/CIDR, aliases, scheduling, `template_id` bypass, status query, balance.

### 1.5 Quick SMS (`/messaging/messages/quick` & `/customer/messages/quick`)
- **Reference Behavior & UI**:
  - Title: `Quick Message`. Card/Section layout with:
    - `Sender ID` (`required`, searchable select of assigned approved senders, auto-selects when user has exactly 1 sender).
    - `Recipient Phone Number` (`PhoneInput` tags `maxItems(1)`, recent 10 numbers suggestions, helper `"Quick Message only supports a single recipient. Use Bulk SMS for multiple recipients."`). Server-side hard reject if `count($phones) > 1`.
    - `Use Bulk SMS Template` (optional dropdown showing template name + `🟢 [Approved]` / `🟡 [Pending Review]` / `🔴 [Rejected]`, auto-fills `body` & recomputes `sms-stats`).
    - `Message` textarea (`placeholder="বাংলায় আপনার এসএমএস লিখুন..."`).
    - Live `sms-stats` bar: `Remaining: X/160` (or `/70`), `SMS Count: N`, `Encoding: GSM-7 | Unicode`, `Non-masking: ৳min–max/SMS`, `Masking: ৳min–max/SMS` + BTRC Bengali compliance banner (`btrc-disclaimer.blade.php`).
    - Buttons: `Send Now` (sync dispatch for Quick SMS, 5-sec duplicate lock, bypasses Bulk SMS approval since single recipient, affordability check, redirects to batch view with toast `Message sent successfully` / `Messages are being processed`) and `Schedule Message` (opens `Schedule Quick Message` modal with `schedule-banner.blade.php` showing timezone + current time, `Select Day`, `Hour`, `Minute`, `AM/PM`, validates future time + required fields + affordability, redirects to batch view). Supports `?schedule=1` auto-opening modal if triggered from Scheduled Messages.
- **Target Gaps**:
  - `frontend/src/routes/customer/messages/quick/+page.svelte:22-27, 88-98`: reads `non_masking_price` & `masking_price`, while `/api/v1/customer/pricing` returns `non_masking_rate` & `masking_rate` -> falls back silently to wrong hardcoded `0.25`/`0.55`.
  - Allows >1 recipient client-side AND `messages.go` Quick SMS handler must strictly reject >1 recipient.
  - Does not open schedule modal on `?schedule=1`.
  - Missing recent recipient phone suggestions & BTRC disclaimer/stats exact format.
- **Status**: `NOT STARTED`
- **How Verified**: Pending end-to-end Quick SMS send & schedule tests (1 phone allowed, 2 phones rejected, live pricing reflected, sync dispatch & redirect).

### 1.6 Bulk SMS (`/messaging/messages/smart`, `/messaging/messages/bulk`, `/customer/messages/bulk`)
- **Reference Behavior & UI**:
  - Title: `Bulk Message`.
  - Admin Approval Warning/Success Callout (visible when `!sms_auto_approved`): amber warning explaining approval required, switching to green callout when an Admin-Approved Template is selected AND body matches it (`messageMatchesTemplate` regex).
  - Form fields: `Sender ID` (auto-select if 1), `Source` (`Manual` | `Group` | `Upload`):
    - `Manual` (no `{vars}`): tags phone input + recent 10 suggestions; (`{vars}` detected in body): dynamic Repeater table (`Phone` + each `{Variable}`).
    - `Group`: searchable multi-select of customer groups; shows warning callout if body has `{variables}` (variables sent literally for groups).
    - `Upload`: CSV/TXT/XLS/XLSX up to 100 MB, Sample file download link (`bd_phone_numbers.xlsx` or CSV), Upload Preview (`X rows · Y columns` + first rows preview table) + Column Mapping component (`phone` + each `{variable}`).
    - `Use Bulk SMS Template`: autofills body + detects `{variables}` + shows approval status helper text.
    - `Message` textarea + `sms-stats` bar + BTRC notice.
  - Actions:
    - Primary button: label switches dynamically to `Submit for Approval` (amber) when approval required, or `Send Now` (primary) when auto-approved or approved-template bypass applies!
    - Clicking opens **"Double-Check & Confirm SMS"** / **"Submit Bulk SMS for Admin Approval"** confirmation modal with: Approval notice (if required), BTRC নির্দেশনা box, Message Preview, 3 KPI tiles (`Recipients`, `SMS Count`, `Rate / SMS`), and breakdown (`Previous balance`, `Total SMS`, `Total cost` / `Estimated cost (held until approval)`, `Balance after sending`).
    - Affordability check is skipped at submission when approval is required (checked at admin approval/dispatch time).
    - `Schedule Message` opens `Schedule Bulk Message` modal (`Select Day`, `Hour`, `Minute`, `AM/PM`) and creates Scheduled batch (respecting `requiresApprovalForCampaign`).
    - Redirects to Batch View (`/messaging/batches/{id}`).
- **Target Gaps**:
  - `frontend/src/routes/customer/messages/bulk/+page.svelte:29-34, 214-224`: `loadPricing()` reads `non_masking_price`/`masking_price` instead of `non_masking_rate`/`masking_rate` -> silent hardcoded fallback.
  - Does not open schedule modal when navigated with `?schedule=1`.
  - Verify upload/group/manual variable mapping, approval callout + approved-template bypass (`messageMatchesTemplate`), confirmation modal breakdown, sample file download, and `batches.template_id` persistence match Reference end-to-end.
- **Status**: `NOT STARTED`
- **How Verified**: Pending live Bulk SMS tests for Manual (plain & variable repeater), Group, Upload (CSV/XLSX mapping + preview), Approved vs Unapproved template bypass, `sms_auto_approved=0`, and Scheduled Bulk.

### 1.7 DLR Webhook Refund Bug & Customer Notifications
- **Reference Behavior & UI**:
  - On gateway DLR failure (`dlr_failed`), credit-refund the message cost back to user wallet (`Transaction` credit with `transactionable_type = Message`, updating `user.balance` and `refund_balance_after`).
  - Database notifications in `notifications` table sent to customer (`notifiable_type = 'App\Models\User'`, `notifiable_id = user.id`, Filament database notification JSON schema `{title, body, icon, iconColor, status, actions, format: 'filament'}`) on:
    - Deposit completed (`CompleteDeposit`) / Admin manual deposit / Admin approval / rejection
    - KYC Approved / Rejected / Resubmission Requested (`ApproveKycAction`, `RejectKycAction`, `RequestKycResubmissionAction`, `SpKycWorkflow`)
    - Sender Approved / Rejected (`ApproveSenderAction`, `RejectSenderAction`)
    - Template Approved / Rejected (`TemplatesTable` admin actions)
    - Batch Approved / Rejected / Completed / Failed (`ApproveAction`, `RejectAction`, `ProcessBatchJob`)
    - IP Whitelist Approved / Rejected / Enabled / Revoked
  - Notification bell in top navbar: unread badge count, slide-over/dropdown list showing title, body, timestamp, unread dot, Mark as Read, Mark All as Read, Clear/Delete.
- **Target Gaps**:
  - `customer-api/internal/httpapi/webhooks.go:40-65`: `handleSmsWebhook` mistakenly calls `payment.CompleteDeposit` instead of `billing.RefundMessageFailed` on DLR failure!
  - Check `notifications.go` & `customer-api` triggers so customer batch completion/failure/template/KYC/deposit notifications match Filament database notification payload & format cleanly.
- **Status**: `NOT STARTED`
- **How Verified**: Pending Go tests for `handleSmsWebhook` DLR failure refund (`balance` + `transactions` credit + `messages.status = 'dlr_failed'`) & notification emission/UI bell tests.

---

## Priority 2 — Navigation, Chrome, Global Search & Full Resource/Page Parity

### 2.1 Navigation Sidebar, Groups, Badges, Back-to-Admin & Impersonation
- **Reference Behavior & UI** (`CustomerNavigationGroup`, `CustomerNavigationManager`, `CustomerPanelProvider`, `back-to-admin-button.blade.php`):
  - Exact Navigation Groups & Order:
    1. Top-level: `Dashboard` (`/customer` or `/`)
    2. `Messaging` (sort 1): `Live Queue` (`/messaging/live-queue`, sort 0), `Sender ID List` (`/messaging/senders`, sort 1), `Bulk SMS Templates` (`/messaging/templates`, sort 2), `Scheduled Messages` (`/messaging/scheduled-messages`, sort 3, **live `info` badge count of `scheduled + paused` batches**), plus Quick Message (`/messaging/messages/quick`) & Bulk Message (`/messaging/messages/smart` / `/bulk`).
    3. `Audience` (sort 2): `Groups` (`/audience/groups`, sort 1), `Contacts` (`/audience/contacts`, sort 2).
    4. `Billing & Reports` (sort 3): `Sent Message Logs` (`/messaging/batches`, sort 1), `Sent Message Details` (`/messaging/messages`, sort 2), `Payment History` (`/billing/deposits`, sort 3), `Transactional Report` (`/billing/transactions`, sort 4), `Pricing Reports` (`/pricing`, sort 5).
    5. `Analytics` (sort 4): `Messaging` (`/analytics/messaging`, sort 1), `Billing` (`/analytics/billing`, sort 2).
    6. `Developer` (sort 5 — **only visible / accessible when `user.api_access === true`**): `API Access` / `Access Tokens` (`/developer/access-tokens`, sort 1), `IP Whitelist` (`/developer/ip-whitelists`, sort 2).
    7. `Account` (sort 6): `KYC Documents` (`/account/kyc`), `Profile` (`/profile`).
  - Topbar: Global Search (`Cmd+K` / `Ctrl+K`), Back to Admin button (`Back to Admin` red gradient pill linking back when impersonating via `?impersonator_token` / impersonation state), Notification Bell + slide-over, Dark/Light Theme Switcher, Custom User Menu (`user-menu.blade.php`: avatar + online dot, role badge `Verified Customer`, status badge `Pending Approval` / `Approved / Active` / `Application Rejected` / `Account Banned`, email, phone, `Available Balance: BDT X.XX`, Theme switcher, Profile link, Sign Out).
- **Target Gaps**:
  - `Sidebar.svelte`: group ordering/labels/icons/`KYC Documents` placement/`api_access` gating on Developer group/Scheduled count badge/active route highlighting vs alias routes (`/customer/*` vs `/messaging/*`, `/audience/*`, `/billing/*`, `/developer/*`, `/analytics/*`).
  - `Navbar.svelte`: dead Apps waffle button, Global Search Cmd+K modal (`CustomerGlobalSearchProvider`), `Back to Admin` button visibility, custom user menu status/balance/role card parity.
  - Developer routes mismatch (`/customer/ip-whitelist` vs `/developer/ip-whitelists`, `/developer/+page.svelte` vs `/developer/access-tokens`).
- **Status**: `NOT STARTED`
- **How Verified**: Pending sidebar + navbar DOM/browser inspection across `api_access=0`, `api_access=1`, impersonation, dark mode, and global search queries.

### 2.2 Global Search (`Cmd+K` / Topbar Search)
- **Reference Behavior & UI** (`CustomerGlobalSearchProvider.php`):
  - Instant matching across: Navigation Menu Items (with group name & URL), `Messages` (`to`, `body` -> `"SMS to {to}"` + Sender/Status/Date), `Contacts` (`name`, `phone` -> `"{name} ({phone})"` + Phone/Group), `Templates` (`name`, `body` -> body preview), `Deposits` (`transaction_id`, `amount`, `payment_method` -> `"Deposit: ৳..."`), `Transactions` (`id`, `description` -> `"Transaction #id"`), `Scheduled Messages` (`body` -> `"Scheduled: ..."`). Note Campaigns omitted per Reference/locked decision.
- **Target Gaps**:
  - `developer.go:675+` implements `/api/v1/customer/search`, but `Navbar.svelte` needs wired `Cmd+K` / topbar global search dropdown modal rendering categorized results & navigating cleanly.
- **Status**: `NOT STARTED`
- **How Verified**: Pending API search test + frontend Cmd+K navigation test.

### 2.3 Dashboard (`/customer` / `/`)
- **Reference Behavior & UI** (`Dashboard.php`, `CustomerStatsOverviewWidget.php`, `customer-stats-overview.blade.php`, `RecentMessagesWidget.php`, `RecentScheduledMessagesWidget.php`):
  - Header Action: `Top Up` (`DepositAction`: modal min BDT 50, max BDT 100,000, reseller balance check, PipraPay redirect).
  - Row 1 — 6 KPI Cards (3 cols on lg, 15s polling, custom styled gradients/icons, clickable URLs):
    1. `Current Balance` (`BDT X,XXX.XX`, `"Available for campaigns"` -> `/billing/deposits`)
    2. `Messages Today` (integer, `"Sent today"` -> `/messaging/messages`)
    3. `Scheduled SMS` (count, `"{n} upcoming in queue"` or `"No upcoming messages"` -> `/messaging/scheduled-messages`)
    4. `Cost Today` (`X.XX BDT`, `"Today's spend"` -> `/billing/deposits`)
    5. `Success Rate` (`X.X%`, `"{n} sent successfully"` -> `/messaging/messages`)
    6. `Failed Today` (integer, `"Send & delivery failures"` -> `/messaging/messages`)
    *(Note: `Total Contacts`, `Groups`, `Templates` are cards 7–9 in `CustomerStatsOverviewWidget.php` lines 134–168 -> 9 cards total in 3×3 grid! Wait: verified 9 cards in `CustomerStatsOverviewWidget.php` L60-169!)*
  - Row 2 — `Recent SMS` table (30s poll, limit 5, `View All Messages` button, columns: `Recipient`, `Sender ID`, `Message Content`, `Status`, `Cost`, `Sent At`; empty state `"No sent messages yet"` / `"Messages you send will appear here."`).
  - Row 3 — `Upcoming Scheduled SMS` table (15s poll, limit 5, `View All Scheduled` button, columns: `Sender ID`, `Recipient`, `Recipients`, `Message Content`, `Scheduled For` + relative diff, `Status`; empty state `"No upcoming scheduled messages"` / `"You do not have any scheduled messages in queue."`).
- **Target Gaps**:
  - Verify `customer/+page.svelte` has all 9 stat cards (3×3 grid), `Top Up` modal action + reseller check, both Recent tables with exact columns/empty states/links & polling.
- **Status**: `NOT STARTED`
- **How Verified**: Pending live dashboard API + UI verification.

### 2.4 My Live Queue (`/messaging/live-queue` & `/customer/live-queue`)
- **Reference Behavior & UI** (`LiveQueuePage.php`, `CustomerLiveQueueStatsWidget.php`, `CustomerLiveMessageStreamWidget.php`, `customer-live-message-stream.blade.php`):
  - Title: `My Live Queue`.
  - 4 KPI cards (5s poll, last 5 minutes): `⚡ In Queue Now` (`pending + sending`), `📊 Msgs / Min`, `🔄 Active Batches` (`processing`), `✅ Live Success` (`X%` + `success/total delivered (5 min)`).
  - Active Bulk Batches Monitor (only visible when active batches exist): `#id`, pulsing status badge (`PENDING`/`PROCESSING`/`COMPLETED`/`FAILED`), sender ID, `percent%`, `time_ago`, animated progress bar, `X of Y recipients processed`, body snippet.
  - Live Message Stream: 3s poll with Pause/Resume button, View Mode toggle (`stream` vs `table`), Limit selector (`10`–`100`, default `30`), Status chips (`All` / `Pending` / `Sending` / `Dispatched`), Type chips (`All` / `Quick` / `Bulk`), Search (`to` / `body`). Default query shows `pending`/`sending` OR completed in last 10s.
- **Target Gaps**:
  - Verify `live-queue/+page.svelte` & `messages.go` Live Queue endpoint match Reference KPI cards, Active Bulk Batches card, filters, stream/table mode, and 10s recent window.
- **Status**: `NOT STARTED`
- **How Verified**: Pending live-queue polling + filter + view toggle tests.

### 2.5 Sender ID List (`/messaging/senders` & `/customer/senders`)
- **Reference Behavior & UI** (`SenderResource.php`, `SendersTable.php`):
  - Scoped to `sender_user` (`user_id = auth.id()`).
  - Columns: `Sender ID` (searchable), `Type` (`badge`: Masking / Non-Masking per `SenderType`).
- **Target Gaps**:
  - Verify `customer/senders/+page.svelte` & `resources.go` `senders()` badge labels, search, empty state, and `sender_user` scoping.
- **Status**: `NOT STARTED`
- **How Verified**: Pending API + UI table check.

### 2.6 Bulk SMS Templates (`/messaging/templates` & `/customer/templates`)
- **Reference Behavior & UI** (`TemplateResource.php`, `TemplatesTable.php`, `TemplateForm.php`, `TemplateInfolist.php`, `CreateTemplate.php`, `EditTemplate.php`):
  - Callout on Create/Edit: `"Admin Approval & Instant Sending Workflow"` explaining template lock upon approval & instant campaign dispatch benefit.
  - Table columns: `Template Name` (bold, searchable, sortable), `Message Template` (limit 60, tooltip, searchable), `Status` (`approval` badge with tooltip: rejection reason / `"Approved by Admin — Locked and ready for sending"` / `"Awaiting Admin Approval"`), `Created At` (`d M Y, h:i A`, sortable).
  - Filter: `Approval Status` (`SmsApproval`: `pending`, `approved`, `rejected`).
  - Actions: `View` (modal/infolist showing `Template Name`, `Approval Status`, `Message Body`, `Rejection Reason` if rejected, `Created At`, `Approved At`), `Edit` (only visible & allowed when `!isApproved()`; otherwise blocked with `"Template Locked"` warning), `Delete`, Bulk Delete / Force Delete / Restore.
  - Form: `Template Name`, `Message Body` (`placeholder="বাংলায় আপনার এসএমএস লিখুন..."` + live `{var}` extraction + `sms-stats` & BTRC notice), `Variables / Placeholders` tags.
  - On Create: if `sms_auto_approved == 0`, set `approval = 'pending'`, `approved_at = null` and show warning toast `"Bulk SMS Template Submitted for Approval"`; else `approval = 'approved'`, `approved_at = now()` and show `"Bulk SMS Template Created"`.
- **Target Gaps**:
  - `sms_panel.templates` missing `approval`, `approved_by`, `approved_at`, `rejection_reason` columns -> blocking SQL queries in `resources.go`!
  - `frontend/src/routes/customer/templates/+page.svelte:378-396`: Pagination footer lacks Prev/Next page buttons! Also verify BTRC notice, Approval status filter, lock-when-approved UI guard, `{variable}` auto-detection, and View Infolist match Reference.
- **Status**: `NOT STARTED`
- **How Verified**: Pending DB schema fix + CRUD/lock/approval/pagination tests.

### 2.7 Scheduled Messages (`/messaging/scheduled-messages` & `/customer/scheduled-messages`)
- **Reference Behavior & UI** (`ScheduledMessageResource.php`, `ListScheduledMessages.php`):
  - Scoped to `status IN ('scheduled', 'paused')`, sorted `scheduled_at ASC`, polls every `5s`.
  - Tabs with counts: `All Scheduled`, `Active Schedule` (`scheduled`, info), `Paused` (`paused`, warning).
  - Columns: `Sender ID` (badge, searchable), `Recipient Number` (first 2 phones + `(+N more)`, copyable badge, searchable), `Source` (`manual`/`group`/`upload` badge), `Recipients` (numeric), `Message Content` (limit 55, tooltip, searchable), `Scheduled For` (`d M Y, h:i A` + relative diff), `Status` (badge), `Created` (toggleable hidden).
  - Filter: `Status` (`Scheduled` / `Paused`).
  - Actions: `Edit` (modal: `Scheduled Date & Time` min now, `Sender ID`, `Recipient Phone Numbers` editable when manual, `Message Content`; toast `"Scheduled Message Updated"`), `Pause` (when `scheduled`, confirmation modal -> `"Scheduled Message Paused"`), `Resume` (when `paused`, confirmation modal -> `"Schedule Resumed"`), `Delete` (confirmation modal -> `"Scheduled Message Deleted"`), Bulk Delete.
- **Target Gaps**:
  - Verify `scheduled-messages/+page.svelte` links (`New` Quick/Bulk -> `?schedule=1` auto-opening modal), tabs/badges, Edit recipients/time, Pause/Resume/Delete, and 5s polling match Reference.
- **Status**: `NOT STARTED`
- **How Verified**: Pending schedule create/edit/pause/resume/delete tests & UI tab verification.

### 2.8 Sent Message Logs / Batches & Batch View (`/messaging/batches`, `/customer/batches`, `/customer/batches/[id]`)
- **Reference Behavior & UI** (`BatchResource.php`, `ListBatches.php`, `ViewBatch.php`, `BatchInfolist.php`, `MessagesRelationManager.php`):
  - **Note**: In Reference, `BatchResource` (`Sent Message Logs`, `navigationSort = 1` in Billing & Reports) overrides `table()` in `ListBatches.php` to query `Message::whereNotNull('batch_id')`! Columns: `Recipient Number`, `Content`, `Sender ID`, `SMS Type` (`Promotional`/`Transactional`), `Cost` (`money('BDT')`, 4 decimals on export), `Previous Balance`, `Next Balance`, `SMS Count`, `Status`, `Failure Reason`, `Date & Time` (`delivered_at ?? sent_at ?? failed_at ?? created_at`). Polls 30s, filters `Sender` (accessible sender IDs), `Status` (`MessageStatus`), `Created From`/`Until`, Header `Export As` modal (CSV / PDF). Clicking a row opens `MessageResource` View.
  - **Batch Detail (`/batches/{id}`)**: `BatchInfolist` (`Message` section: Template/body, Recipient Phone Numbers, Status, Source, Sender ID; `Message Statistics`: Pending, Sent, Delivered, Failed counts; `Import Statistics`: Total, Valid, Invalid + **"Download invalid list"** button if `failed_report_path` & `invalid_count > 0`, Progress %, Start/Completion/Created timestamps) + `MessagesRelationManager` table of messages in that batch.
- **Target Gaps**:
  - `customer/batches/+page.svelte` vs `messages.go` `batches()`: Verify whether `Sent Message Logs` (`batches`) shows batch-level rows vs Reference `ListBatches` message-level logs (or check what Target currently implements and align!), `next_balance` & 4-decimal cost formatting, sender filter parameter mismatch (`sender_id` numeric ID vs string in `batches/+page.svelte`), export modal, and `Download invalid list` route/handler.
- **Status**: `NOT STARTED`
- **How Verified**: Pending Sent Message Logs + Batch Detail + invalid-list download + CSV/PDF export verification.

### 2.9 Sent Message Details & View Message (`/messaging/messages`, `/customer/messages`, `/customer/messages/[id]`)
- **Reference Behavior & UI** (`MessageResource.php`, `ListMessages.php`, `ViewMessage.php`, `MessageInfolist.php`):
  - Title: `Sent Message Details`, polls `30s`, grouped by `status` (collapsible).
  - Header Actions: `Quick Message` -> create-quick, `Bulk Message` -> create-smart, `Export As` modal (Excel/CSV & PDF `sent-messages-*.csv`/`.pdf`).
  - **19 Columns**:
    1. `Message ID` (`MSG-XXXXXX`, searchable by numeric ID)
    2. `Recipient Number` (`to`, searchable)
    3. `Content` (`body` limit 50, tooltip, searchable)
    4. `Sender ID` (`sender.sender_id`, searchable, placeholder `—`)
    5. `SMS Type` (`Transactional` [green] / `Promotional` [amber] from payload or batch count)
    6. `SMS Count` (`parts`, numeric right-aligned)
    7. `Cost` (`BDT`, right-aligned)
    8. `Previous Balance` (`batch_balance_before`, BDT)
    9. `Next Balance` (`refund_balance_after ?? batch_balance_after`, BDT)
    10. `Status` (`badge`, sortable)
    11. `Failure Reason` (only for `failed`/`dlr_failed`, falls back to `sms_logs.error_message`)
    12. `Operator` (from `sms_logs.operator_name` or prefix lookup Grameenphone/Robi/Airtel/Teletalk/Banglalink)
    13. `Channel` (`CH-01`/`CH-02`/`CH-03` from `billMsisdn` or operator)
    14. `Submit Time` (`sent_at ?? created_at` `h:i A`, sortable)
    15. `Delivery Time` (`delivered_at` `h:i:s A`, placeholder `—`)
    16. `Tariff` (`rate`, BDT right-aligned)
    17. `API User` (`{username}_API` when present in `request_payload`)
    18. `Source IP` (`message.source_ip ?? batch.source_ip ?? batch.source_config.ip ?? payload`)
    19. `Request ID` (`REQ-{last 6 digits}` from `gateway_message_id` or payload)
  - Filters (`AboveContent`): `Sender`, `Status` (`MessageStatus`: `pending`, `sending`, `sent`, `delivered`, `failed`, `dlr_failed`), `Created From`, `Created Until`.
  - View Action (`MessageInfolist`): modal/page showing `UID`, `Sender`, `Recipient`, `Status`, `Cost`, `Parts`, `Sent At`, `Failed Reason`, `Body`, `Created At`, `Updated At`.
- **Target Gaps**:
  - `customer/messages/+page.svelte` only has 12 columns vs Reference 19 columns, lacks View Message Infolist modal/page & `dlr_failed` status option parity, and `export.ts:235` PDF footer says `White Info SMS Admin Platform` instead of `Sendora SMS Platform` (`messages-report.blade.php:118`).
- **Status**: `NOT STARTED`
- **How Verified**: Pending API payload check + all 19 columns + filters + CSV/PDF export + View Infolist check.

### 2.10 Audience — Groups & Contacts (`/audience/groups`, `/audience/contacts`, `/customer/groups`, `/customer/contacts`)
- **Reference Behavior & UI**:
  - `GroupResource` (`navigationSort = 1`): Columns: `Name` (searchable), `Contacts` (`contacts_count` badge), `Description` (limit 20, tooltip, searchable), `Created At`, `Updated At`. Actions: `Create`, `View` (Infolist + `ContactsRelationManager` with **Add Phone Numbers** bulk paste modal validating BD numbers `^(?:\+?88)?0?1[3-9]\d{8}$` -> `+8801XXXXXXXX`, CSV/XLSX Import Contacts to group, Attach existing contacts, Create contact, Edit, Detach, Delete, Export Contacts), `Edit` (+ RelationManager), `Delete`, Bulk Delete.
  - `ContactResource` (`navigationSort = 2`): Columns: `Name` (searchable, sortable), `Phone` (searchable, sortable), `Created At`. Header actions: `New contact`, `Export Contacts` (CSV/XLSX). Validation: BD phone regex `^(?:\+?88)?0?1[3-9]\d{8}$` + unique per user excluding soft-deleted (`deleted_at IS NULL`). View/Edit shows `GroupsRelationManager` (`Name`, `Contacts`, `Created At` linking to Group).
- **Target Gaps**:
  - Verify `contacts` CRUD/export/regex/unique validation, `groups` `contacts_count`, Group `Add Phone Numbers` bulk paste & file import, Attach/Detach contacts, View/Edit relation parity.
- **Status**: `NOT STARTED`
- **How Verified**: Pending Contact & Group CRUD, BD regex, bulk paste, file import, attach/detach, and export tests.

### 2.11 Billing — Payment History, Transactional Report & Pricing Reports
- **Reference Behavior & UI**:
  - `DepositResource` (`Payment History`, `navigationSort = 3`): Header `Make Deposit` (`DepositAction`: min 50, max 100,000 BDT, reseller SP balance check, PipraPay payment initiation + callback/webhook verification). Columns: `Amount` (BDT, sortable), `Payment Method` (badge, sortable), `Trx ID` (searchable), `Status` (badge, sortable), `Paid At`, `Created At`. Action: `View` (`DepositInfolist`: `Amount`, `Method`, `Status`, `Paid At`).
  - `TransactionResource` (`Transactional Report`, `navigationSort = 4`): Columns: `Reference` (`transactionable_type` badge), `Direction` (`credit`/`debit` badge), `Amount` (BDT), `Balance` (`balance_after`, BDT), `Description` (limit 40, searchable), `Date` (dateTime, sortable). Action: `View` (`TransactionInfolist`: Transaction, Balance Before/After, Details).
  - `Pricing` (`Pricing Reports`, `/pricing`, `navigationSort = 5`): Non-paginated table resolved via `OperatorRateResolver::forUser`: Columns `Country`, `Operator`, `Codes` (badge list), `Masking` (`৳ X.XXXX` or `—`), `Non Masking` (`৳ X.XXXX` or `—`).
- **Target Gaps**:
  - `pricing()` backend bug (`resources.go:1574` queries `is_active` instead of delegating to `OperatorRateResolver` with `status = 'active'`).
  - Verify Deposit View Infolist, Transaction View Infolist, Reseller balance check on Top Up, and PipraPay callback flow.
- **Status**: `NOT STARTED`
- **How Verified**: Pending Deposit/PipraPay/Transaction/Pricing API + UI verification.

### 2.12 Analytics — Messaging & Billing (`/analytics/messaging`, `/analytics/billing`)
- **Reference Behavior & UI**:
  - `HasDateFilter` (shared by both pages): Date range presets: `Today`, `Yesterday`, `This Week`, `Last 7 Days`, `This Month`, `Last 30 Days` (default), `This Year`, `Custom Range` (showing `From` & `To` DatePickers).
  - `MessagingAnalytics`: Header `Export As` modal (CSV `messaging-analytics-*.csv` & PDF `messaging-analytics-*.pdf` with Performance Summary KPI table + Daily Stats table); Widgets:
    - `MessageStatsByPeriodWidget` (7 stat cards, 3 cols, 60s poll): `Total Messages`, `Success Rate` (`sent+delivered`), `Delivered`, `Failed` (`failed+dlr_failed`), `Total Cost` (`status NOT IN ('failed','dlr_failed')`), `Avg Parts`, `Unicode`.
    - `MessageTrendChart` (line), `DeliveryRateTrendChart` (line %). *(Campaign widgets hidden/excluded per locked decision).*
  - `BillingAnalytics`: Header `Top Up` (`DepositAction`). Widgets (2-col grid):
    - `BillingStatsWidget` (4 cards): `Deposits` (completed in period), `Spent` (`status != 'failed'` per Reference `BillingStatsWidget.php:33`), `Pending Deposits` (awaiting approval), `Transactions` (count in period).
    - `DepositTrendChart` (bar), `SpendingTrendChart` (bar), `DepositMethodChart` (doughnut), `TransactionVolumeChart` (line credits vs debits).
- **Target Gaps**:
  - `customer/analytics/+page.svelte` only has 3 presets (`Today`, `Last 7 Days`, `Last 30 Days`) — missing `Yesterday`, `This Week`, `This Month`, `This Year`, `Custom Range` + CSV/PDF export modal matching Reference columns!
  - `analytics/billing/+page.svelte` needs date preset parity + dark mode classes + Top Up button parity.
  - Check `billing_analytics.go` SQL accuracy for all 7/4 KPIs & chart series.
- **Status**: `NOT STARTED`
- **How Verified**: Pending analytics endpoint responses across presets/custom range + UI chart/KPI/export verification.

### 2.13 Developer — Access Tokens & IP Whitelists (`/developer/access-tokens`, `/developer/ip-whitelists`)
- **Reference Behavior & UI**:
  - Both gated behind `user.api_access === true` in nav & access control (`403` when `false`).
  - `AccessTokens`: Header `New access token` modal (`Token Name` required max 255, `Expiration Date` select: `1 Day`, `7 Days`, `1 Month` [default 30], `3 Months`, `6 Months`, `1 Year`, `Never`). After creation, automatically opens **`View Token`** modal (non-dismissible by clicking away, copyable secret, warning callout `"This is the only time you will be able to view or copy this token..."`). Table columns: `Token Name`, `Last Used` (`Never Used`), `Expires At` (`Never Expires`). Actions: `Edit` (rename), `Delete` (**requires typing exact token name confirmation** matching Reference `AccessTokensTable.php:48-72`), `Bulk Delete`.
  - `IpWhitelists`: Header `Request IP Whitelist` modal (`Public IP Address / CIDR Block` required, strict validation rejecting private/RFC1918/loopback/reserved/CGNAT `100.64.0.0/10`/`0.0.0.0/8`, IPv4 `/16–/32`, IPv6 `/48–/128`; `Label / Description` optional). Sets `status = 'pending'` & logs `IpWhitelistAuditLog`. Table columns: `IP Address` (bold, copyable), `Label`, `Status` (badge), `Admin Remark`, `Requested At`, `Reviewed At`. Actions: `Edit` (if `ip_address` changed, resets `status = 'pending'`, clears `admin_note`, shows warning `"IP Address Updated"`, logs `IpWhitelistAuditLog`), `Remove` (`DeleteAction` confirmation, logs `IpWhitelistAuditLog`).
- **Target Gaps**:
  - `resources.go` vs `developer.go`: route overlap / missing token expiry options & rename/confirm-name-to-delete; `resources.go` `ipWhitelist` lacks public-IP/CIDR validation, edit action (reset to pending on IP change), and `ip_whitelist_audit_logs` writes (whereas `developer.go` may differ — unify frontend to the complete `developer.go` + enforce all Reference validations & audit logs!).
  - Route mismatch: `/customer/ip-whitelist` vs `/developer/ip-whitelists`, `/developer` vs `/developer/access-tokens`.
- **Status**: `NOT STARTED`
- **How Verified**: Pending AccessToken lifecycle (create with expiry + one-time modal + rename + exact-name delete) & IPWhitelist validation (reject `192.168.1.1`/`10.0.0.1`/`127.0.0.1`/`100.64.1.1`/`8.8.8.0/8`, allow `203.0.113.10`/`198.51.100.0/24`) + `ip_whitelist_audit_logs` DB check.

### 2.14 Exports & Reports Branding (`export.ts` + CSV/PDF parity)
- **Reference Behavior & UI**:
  - Report PDF footer says `Generated by Sendora SMS Platform` (`messages-report.blade.php:118`), not Admin branding. Date format, currency `BDT`, and column sets match Reference CSV/PDF exporters.
- **Target Gaps**:
  - `frontend/src/lib/utils/export.ts:235`: PDF footer says `White Info SMS Admin Platform`.
- **Status**: `NOT STARTED`
- **How Verified**: Pending CSV & PDF export inspection.
