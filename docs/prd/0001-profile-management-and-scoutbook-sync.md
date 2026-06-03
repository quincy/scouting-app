# PRD: Profile Management & Scoutbook Sync

## Problem Statement

The app currently supports event management with a seeded admin user, but has no way for real troop members to register or sign in. All user profile data (names, BSA IDs, emails) lives only in Scoutbook — the troop's external roster system — with no way to import it. Parents cannot manage their children's attendance. The system cannot distinguish between adult and youth members.

## Solution

Introduce a **Profile** domain model synced from Scoutbook via Bearer token, an **email-based OTP registration** flow for claiming Profiles, a **parent-youth linking** system with admin approval, and **Profile-based event attendance** so parents can enrol multiple linked youths. No self-serve user sign-up exists — only users with a pre-existing Profile in Scoutbook can register.

## User Stories

1. As an **admin**, I want to paste my Scoutbook Bearer token into the app, so that the roster can be imported without building a full proxy auth flow.
2. As an **admin**, I want the sync to call Scoutbook's `orgAdults` and `orgYouths` APIs, deduplicate members by BSA ID, and optionally fetch email via `personprofile`, so that all troop members appear as Profiles in the app.
3. As an **admin**, I want profiles that no longer appear in Scoutbook to be marked inactive (not deleted), so that historical attendance data is preserved.
4. As an **admin**, I want to see a list of pending parent-youth link requests and approve or reject them, so that only legitimate connections are established.
5. As an **admin**, I want to configure the organization GUID via an environment variable, so that the sync knows which Scoutbook unit to pull from.
6. As a **troop member**, I want to register with my email address, so that I can claim my Scoutbook Profile and access the app.
7. As a **troop member**, I want to receive a one-time passcode (OTP) by email to verify I own the address, so that my identity is confirmed before I create a password.
8. As a **troop member**, if I try to register with an email not found in any Profile, I want to be redirected to the login page with a message to contact an admin, so that I know I'm not in the roster yet.
9. As a **troop member**, if I try to register with an email that already has a User linked, I want to be redirected to the login page with a "already registered" message, so that I don't create a duplicate account.
10. As a **parent**, after registering, I want to claim my child's Profile by entering their BSA ID, so that I can manage their attendance.
11. As a **parent**, I want to see my own name and my linked youths' names listed on the event detail page with individual "Sign Up" / "Withdraw" buttons, so that I can enrol or withdraw each person independently.
12. As a **parent**, I want to sign up a linked youth for an event on their behalf, so that the youth can participate even without their own account.
13. As a **user**, I want to log in using my email and password, so that I can access the app securely.
14. As a **user**, I want to see which profiles I'm attending as in the attendee list, so that I can track my participation.

## Implementation Decisions

### Domain Model

- **Profile** replaces User as the entity carrying PII. Fields synced from Scoutbook: `bsa_id` (memberId), `first_name`, `last_name`, `email`, `phone`, `birthdate`, `member_type` (adult/youth), `status` (active/inactive). Linked to User via `user_id` (nullable, 0..1).
- **User** loses the `email` column. Login flow: find Profile by email → get linked User → verify password hash.
- **ParentYouthLink** join table: `parent_profile_id`, `youth_profile_id`, `status` (pending/approved/rejected), `requested_at`, `approved_at`, `approved_by`.
- **OTPCode** table: `email`, `code` (6-digit), `expires_at`, `used`, `created_at`.
- **ScoutbookSession** table: `token` (encrypted JWT), `person_guid`, `expires_at`, `created_at`.

### Attendance Migration

- `event_attendees.user_id` → `event_attendees.profile_id` (profile-based attendance).
- `event_attendee_responsibilities.user_id` → `event_attendee_responsibilities.profile_id`.
- `EventRepository.SignUp/Withdraw/GetAttendees` accept `profile_id` instead of `user_id`.
- `EventListItem.AttendeeCount` remains profile-count.

### Scoutbook Sync Flow (Token Paste)

1. Admin pastes `LOGIN_DATA` JSON from `advancements.scouting.org` localStorage (or the `token` cookie).
2. App decodes and stores the JWT and `personGuid` in `scoutbook_sessions`.
3. Sync calls `POST /organizations/v2/{orgGuid}/orgAdults` and `POST /organizations/v2/{orgGuid}/orgYouths` concurrently with `{"includeRegistrationDetails":true,"includeExpired":true}`.
4. Deduplicates by `memberId` (BSA ID). Members in `orgAdults`-only → adult; `orgYouths`-only → youth; both → adult.
5. For each unique member, optionally calls `GET /persons/v2/{personGuid}/personprofile` (in concurrent batches) to fetch email, phone, birthdate.
6. Updates local Profiles: creates new, updates existing, marks missing as inactive.
7. Email sync rule: Scoutbook email replaces local if different; local email preserved if Scoutbook has none.

### Registration Flow

1. `GET /register` → email input form.
2. `POST /register` → lookup Profile by email. If 0 → redirect to login with "contact admin". If already linked → redirect to login with "already registered".
3. Generate 6-digit OTP, store to `otp_codes`, send via SMTP.
4. `GET /register/verify` → OTP input form.
5. `POST /register/verify` → validate OTP, mark used.
6. `GET /register/complete` → password creation form.
7. `POST /register/complete` → create User, link to Profile, create session, redirect to events.

### Parent-Youth Claim Flow

1. Registered parent visits `/claim-youth`.
2. Enters youth's BSA ID.
3. System validates no existing User linked, and youth Profile exists.
4. Creates `parent_youth_links` record with `status: pending`.
5. Admin reviews in `/admin/links` and approves/rejects.
6. Once approved, parent sees youth in their event detail profile list.

### Event Detail UI

- Replaces single Sign Up/Withdraw button with a list of manageable profiles.
- For unlinked adult user: shows their own name with button.
- For linked parent: shows their own name + each linked youth's name, each with individual Sign Up/Withdraw button.
- HTMX partials: `profile_signup_list.html` replaces `signup_button.html`; `attendee_list.html` shows profile names instead of emails.

### SMTP Configuration

- New env vars: `SMTP_HOST`, `SMTP_PORT`, `SMTP_USER`, `SMTP_PASS`, `SMTP_FROM`.
- `EmailService` interface with `SendOTP(recipient, code)` method.
- Implementation sends via SMTP.

## Testing Decisions

**What makes a good test:**
- Tests exercise public interfaces (domain methods, handler responses, repository methods).
- Domain logic (OTP generation, dedup, email sync rules) tested with in-memory repos.
- Handler responses tested via `httptest` — check status codes, redirects, and key content.
- No database required; all tests use `mock` repositories.

**Modules to test:**
- `domain`: Profile creation, OTP validation, email sync rules, parent-youth link state machine.
- `api`: Registration handler flow (GET/POST each step), claim flow, sync admin page, event detail rendering with multiple profiles.
- `storage/mock`: New ProfileRepository, OTPRepository, ScoutbookSessionRepository, ParentYouthLinkRepository CRUD.

**Prior art:** Existing tests in `internal/domain/auth_test.go`, `internal/api/auth_test.go`, `internal/api/events_test.go`, `internal/storage/mock/event_test.go`.

## Out of Scope

- Full Scoutbook proxy auth flow (admin enters credentials in our app). Token paste is the M2 approach.
- Profile picker on login for shared emails (parent and youth with same email). Deferred to later milestone.
- Youths claiming their own account by unlinking from a parent. Deferred.
- CSV/export fallback import for Scoutbook data.
- Password reset flow.
- Email branding/template customization — plain-text OTP email with app name.

## Further Notes

- Scoutbook uses reCAPTCHA, making proxy auth significantly harder. Token paste avoids this entirely.
- The `memberId` (BSA ID) is the dedup key. Same person can appear in both `orgAdults` and `orgYouths` (they are adults with youth-program roles). Resolve as adult.
- The org GUID (`CE0212A8-...`) is configured via `SCOUTBOOK_ORG_GUID` env var.
- `LOGIN_DATA` is stored in localStorage at `advancements.scouting.org`. The JWT also lives in a `token` cookie on that domain.
- JWT Bearer token format: `eyJ...` (starts with `Bearer ` prefix in the API call but the stored value in LOGIN_DATA has it as-is).
