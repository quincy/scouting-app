# Context: Scout App

This document defines the ubiquitous language for the scout troop event organization app.

## Glossary

### Event
A planned social or troop activity with a title, description, location, and timing.

### Campout
A specific type of **Event** that typically spans multiple days.

### User
A security principal authenticated via password. A User has no PII and no inherent email — personal information lives on the linked **Profile**. Users have **Roles** that determine their **Permissions**. Roles are assigned from three sources: **Registration** assigns **Parent** or **Scouts BSA**; **Scoutbook Sync** reconciles position-derived roles; and **Admin** can assign privileged roles like **Admin**. A User links to exactly one **Profile**.

### Profile
A PII record synced from Scoutbook, identified by its **BSA ID**. Contains first name, last name, nickname, email, phone, birthdate, **Member Type**, **Positions**, and **Status**. A Profile links to exactly zero or one **User**. When linked, the Profile is "registered." Profiles are the entities that sign up for events as **Attendees**.

### Profile Status
Indicates whether a **Profile** is currently part of the unit. Set to `inactive` by **Scoutbook Sync** when a profile's **BSA ID** no longer appears in the Scoutbook roster; set back to `active` if the profile reappears in a future sync. A profile is always created as `active`. The status controls what operations are permitted:
- **Active**: All normal operations allowed (sign-up, login, registration, etc.).
- **Inactive**: Cannot sign up for events, cannot log in (admins bypass login restriction), cannot register a new **User**. Withdrawing from events is only possible by a user with the **Admin** role. Inactive profiles are hidden from event sign-up dropdowns. The **Admin Roster** page displays status with a color-coded badge and supports filtering by status.

### Display Name
A formatted name for a **Profile**, returned by `Profile.DisplayName()`. When the Profile has a non-empty **Nickname**, the format is `Nickname (FirstName) LastName`. Otherwise it falls back to `FirstName LastName`. Used consistently across all UI contexts.

### Role
A designation assigned to a **User** that determines their permissions. A **User** can have multiple **Roles**. Roles come from three sources:
- **Status roles** — assigned at **Registration** (e.g., **Parent** for adults). Never touched by **Scoutbook Sync**.
- **Position-based roles** — derived from a **Profile**'s **Positions** field. Reconciled every **Scoutbook Sync**: roles matching current positions are added, roles no longer held are removed.
- **Privileged roles** — assigned by an **Admin** (e.g., **Admin**). Never touched by **Scoutbook Sync**.
A **User** has a permission if **any** of their roles grants it.

### Permission
A specific action that a **User** is allowed to perform (e.g., `event:create`, `event:signup`). Permissions are mapped to **Roles** via a run-time admin interface.

### Attendee
A **Profile** that has been signed up to participate in a specific **Event**. An Attendee has a status (`signed_up`, `canceled`) and may hold one or more **Responsibilities** for that Event.

### Attendee Status
Indicates whether an **Attendee** is currently participating (`signed_up`) or has been removed (`canceled`).

### Responsibility
A designated function an **Attendee** holds for a specific **Event** (e.g., `driver`, `spl`, `coordinator`, `medical_officer`, `cook`). An Attendee may hold multiple responsibilities. A `driver` responsibility includes a **Seatbelt Count** indicating the total number of seatbelts in that driver's vehicle (including the driver's own seatbelt). Some responsibilities are **Singleton** — only one attendee can hold them per event.

### Coordinator
A **Singleton** **Responsibility** for an **Event**, auto-assigned to the event creator. Coordinates the event logistics. Reassignable by anyone with `event:create` permission.

### SPL (Senior Patrol Leader)
A **Singleton** **Responsibility** for an **Event**, auto-assigned on sign-up to a **Youth** attendee whose **Profile** holds the `Senior Patrol Leader` **Position**. Reassignable by anyone with `event:create` permission.

### Medical Officer
A **Singleton** **Responsibility** for an **Event**, assignable by anyone with `event:create` permission.

### Singleton
A **Responsibility** that only one **Attendee** can hold per **Event**. Reassigning a singleton to a different attendee requires confirmation and automatically removes it from the previous holder.

### Seatbelt Count
The total number of seatbelts available in a **Driver**'s vehicle, inclusive of the driver's own seatbelt. Stored on the **Driver** responsibility record. Used to compute the event's **Available Seatbelts**.

### Driver
An **Attendee** who holds the `driver` **Responsibility** for an **Event**. Each Driver provides a specific number of **Seatbelt Count** for their vehicle. Assignable by the driver themself or by anyone with `event:create` permission.

### Available Seatbelts
The sum of all **Seatbelt Counts** across all **Drivers** for an **Event**. Must be >= the number of **Attendees** (Required Seatbelts) for the event to have sufficient capacity.

### Required Seatbelts
The total number of **Attendees** (with `signed_up` status) for an **Event**. Each attendee consumes one seatbelt.

### Event Type
A classification of an **Event** (e.g., `campout`). Defined as a fixed set of known values.

### Sign-up
The action of registering a **Profile** as an **Attendee** for an **Event**. A **User** may sign up their own **Profile** or any **Profile** linked via **Parent Youth Connection**.

### Withdraw
The action of removing a **Profile** from the **Attendee** list for an **Event**.

### Active Event
An **Event** whose end time has not yet passed.

### Past Event
An **Event** whose end time has passed.

### Position
A Scoutbook-assigned title held by a **Profile** (e.g., `Scoutmaster`, `Patrol Leader`, `Scribe`). Stored as a comma-separated string on the **Profile**. Each **Position** corresponds to a **Role** of the same name. When a **Profile** is linked to a **User**, the **Scoutbook Sync** reconciles the user's position-based roles to match their current positions.

### Event Cost
The amount in currency required for a **User** to participate in an **Event**. For the MVP, this is a fixed value per **Event** for informational purposes.

### Event Communication
A message sent to **Profiles** about an **Event**. Two types: `announcement` (sent once after event creation) and `reminder` (sent later, may include an additional message). Sent by a user with `event:create` permission. The email body contains the event's description rendered as HTML (via **goldmark**). The **Reply-To** header is set to the **Coordinator**'s profile email.

### Announcement
An initial **Event Communication** notifying the **Troop** about a newly created **Event**. Subject format: `[{UnitType} {UnitNumber}] {EventType} {EventTitle}`. Can only be sent once; subsequent sends are **Reminders**.

### Reminder
A follow-up **Event Communication** sent after the **Announcement**. May include an additional markdown message above the event description. Can be sent to either all **Attendees** or the entire **Troop** (all active **Profiles** with an email).

### Sent Communication
A record of a sent **Event Communication**, stored in `event_communications` with the subject, HTML body, additional message (if any), sender, recipient scope, and timestamp. Recipient **Profile** IDs are stored in `event_communication_recipients`. Displayed in the **Event** detail page with recipient names (never email addresses).

### Upcoming Events
A chronological list of **Active Events** (future events).

### Event Archive
A chronological list of **Past Events** (historical events).

### EventListItem
An **Event** summary projected for list views, containing the core event fields plus the number of signed-up **Attendees**.

### Registration
The process by which an unregistered **Profile** becomes linked to a **User**. Three-step flow: email entry and **OTP** generation, OTP verification, and password creation. The OTP email includes a link to `/register/verify?otp_id=<uuid>`, using the OTP record's UUID to identify the user (not their email). The unauthenticated **Session** tracks progress — the email is stored after OTP generation, and a `verified_email` flag is set after successful OTP validation. If no **Profile** exists for the email, the user is told no account was found and directed to check their Scoutbook email or contact the Troop Webmaster. If the **Profile** is already linked to a **User**, the user is shown an error with a link to the login page. After password creation, the user is redirected to `/login?registered=1` which displays a persistent success banner. When the **User** is created, they are assigned the **Role** `parent` if their **Profile** has **Member Type** `adult`. If the **Profile** already has **Positions** at registration time, those are also assigned as **Roles** (same logic as **Scoutbook Sync**). Position-based roles are later kept in sync by subsequent **Scoutbook Sync** runs.

### App Config
A key-value table storing application-wide configuration set during onboarding. Expected keys: `SCOUTBOOK_ORG_GUID`, `UNIT_TYPE`, `UNIT_NUMBER`, `DEFAULT_TIMEZONE`, `ONBOARDING_COMPLETE`. Replaces the corresponding environment variables from the previous configuration scheme.

### Onboarding
The first-run setup flow that creates the initial **Profile**, **User**, and **App Config** when no profiles exist. Multi-step, welcoming flow with step indicators showing progress. Skips **OTP** — the admin is trusted by having deploy access. Does not persist partial state; closing mid-flow means starting over. Creates the first profile with **Admin** **Role**. Guarded by the `ONBOARDING_COMPLETE` key in **App Config**. The config values formerly set via environment variables (`SCOUTBOOK_ORG_GUID`, `UNIT_TYPE`, `UNIT_NUMBER`) are collected during onboarding and stored in **App Config** instead.

### Authentication
The process of verifying a **User**'s identity by finding a **Profile** by email, resolving the linked **User**, and checking the provided password against the stored **Password Hash**.

### Hasher
An abstraction over password hashing that can **Hash** a plaintext password and **Verify** a password against an existing **Hash**.

### Password Hash
The bcrypt hash of a **User**'s password, stored on the **User** record. Never stored in plaintext.

### Session
A server-side record of an authenticated **User**'s login, tracked via an encrypted cookie (`session`) and stored by `gorilla/sessions`. Sessions expire after 24 hours.

### BSA ID
The unique identifier for a **Profile**, sourced from Scoutbook's `memberId` field. Used to deduplicate members during sync and to link **Profiles** to **Users**.

### Member Type
A classification of a **Profile** as either `adult` or `youth`. Determined during sync by which Scoutbook endpoint returned the member (`orgAdults` or `orgYouths`). Members appearing in both lists resolve as `adult`.

### Registered Profile
A **Profile** that has been linked to a **User** via the **Registration** flow. Indicates the person has an active app account. An adult registers their own Profile via email verification (**OTP**) and password creation. A youth cannot register directly; a parent links to the youth's Profile via the **Parent Youth Connection** flow. The **Admin Roster** page displays a "registered" / "not registered" badge.

### Parent Youth Connection
A join record connecting a parent's **Profile** to a youth's **Profile**, allowing the parent to sign up or withdraw the youth for **Events**. Has status `pending`, `approved`, `rejected`, or `revoked`. Requires admin approval to activate. An admin can revoke an active connection from the Connections Manager.

### OTP (One-Time Passcode)
A 6-digit code sent via email to verify a user's identity during **Registration**. Stored in `otp_codes` with an expiry timestamp (30 minutes) and a `used` flag. Requesting a new OTP invalidates any existing unused OTP for the same email. Rate-limited to 5 requests per hour per email (counted by existing OTP records in that window). Expired OTP codes are cleaned up by a background goroutine that runs every 24 hours. After the OTP is verified and marked used, the user proceeds to password creation. The OTP email includes a link to `/register/verify?otp_id=<uuid>` so the user can navigate directly from their email.

### Scoutbook Session
An encrypted record of a Bearer JWT token obtained from Scoutbook, stored so the app can call the Scoutbook API on behalf of an admin. Includes the `personGuid`, `expires_at` timestamp, and the encrypted token.

### Scoutbook Sync
The process of importing roster data from Scoutbook into the app's **Profile** table. An admin pastes their Bearer JWT (obtained from the SPA at `advancements.scouting.org`), and the app calls the Scoutbook API at `api.scouting.org` (`POST /organizations/v2/{orgGuid}/orgAdults` and `POST /organizations/v2/{orgGuid}/orgYouths` with body `{includeRegistrationDetails:true, includeExpired:true}`), deduplicates by **BSA ID**, fetches email via `personprofile`, and upserts local **Profile** records. Profiles that no longer appear in Scoutbook are marked `inactive`.

When a **Profile** is linked to a **User** and its **Positions** have changed, **Scoutbook Sync** reconciles the user's position-based **Roles** to match. Roles matching current positions are added; position-based roles no longer held are removed. **Status roles** (e.g., **Parent**) and **Privileged roles** (e.g., **Admin**) are never touched.

## UI Conventions

### Adult/Youth Separation
Whenever a list of **Profiles** is displayed, **Adults** and **Youth** must appear as two separate, clearly-labeled sections (never combined into a single flat list). Each entry should be identifiable by its section as an adult or a youth without needing additional badges or labels. Examples: the Admin Roster page uses collapsible/hidden sections, the Admin Roles page uses consecutive sections. This applies to all profile listings, including HTMX partials. Stick to the language "Adults" and "Youth / Scouts" for section headers throughout the UI.
