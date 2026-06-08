# Context: Scout App

This document defines the ubiquitous language for the scout troop event organization app.

## Glossary

### Event
A planned social or troop activity with a title, description, location, and timing.

### Campout
A specific type of **Event** that typically spans multiple days.

### User
A security principal authenticated via password. A User has no PII and no inherent email — personal information lives on the linked **Profile**. Users have **Roles** that determine their **Permissions**. A User links to exactly one **Profile**.

### Profile
A PII record synced from Scoutbook, identified by its **BSA ID**. Contains first name, last name, nickname, email, phone, birthdate, and **Member Type**. A Profile links to exactly zero or one **User**. When linked, the Profile is "claimed." Profiles are the entities that sign up for events as **Attendees**.

### Display Name
A formatted name for a **Profile**, returned by `Profile.DisplayName()`. When the Profile has a non-empty **Nickname**, the format is `Nickname (FirstName) LastName`. Otherwise it falls back to `FirstName LastName`. Used consistently across all UI contexts.

### Role
A designation assigned to a **User** that determines their permissions. A **User** can have multiple **Roles**.

### Permission
A specific action that a **User** is allowed to perform (e.g., "Create Event", "Sign up for Event"). Permissions are mapped to **Roles**.

### Attendee
A **Profile** that has been signed up to participate in a specific **Event**. An Attendee has a status (`signed_up`, `canceled`) and may hold one or more **Responsibilities** for that Event.

### Attendee Status
Indicates whether an **Attendee** is currently participating (`signed_up`) or has been removed (`canceled`).

### Responsibility
A designated role an **Attendee** holds for a specific **Event** (e.g., `driver`, `cook`). An Attendee may hold multiple responsibilities.

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

### Event Cost
The amount in currency required for a **User** to participate in an **Event**. For the MVP, this is a fixed value per **Event** for informational purposes.

### Upcoming Events
A chronological list of **Active Events** (future events).

### Event Archive
A chronological list of **Past Events** (historical events).

### EventListItem
An **Event** summary projected for list views, containing the core event fields plus the number of signed-up **Attendees**.

### Registration
The process by which an unclaimed **Profile** becomes linked to a **User**. Three-step flow: email entry and **OTP** generation, OTP verification, and password creation. The OTP email includes a link to `/register/verify?otp_id=<uuid>`, using the OTP record's UUID to identify the user (not their email). The unauthenticated **Session** tracks progress — the email is stored after OTP generation, and a `verified_email` flag is set after successful OTP validation. If no **Profile** exists for the email, the user is told no account was found and directed to check their Scoutbook email or contact the Troop Webmaster. If the **Profile** is already linked to a **User**, the user is shown an error with a link to the login page. After password creation, the user is redirected to `/login?registered=1` which displays a persistent success banner. When the **User** is created, they are assigned the **Role** `parent` if their **Profile** has **Member Type** `adult`, or `scout` if their **Profile** has **Member Type** `youth`.

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

### Claim
The action of linking a **Profile** to a **User**, establishing ownership. An adult claims their own Profile via email verification (OTP) and password creation. A parent claims a youth's Profile via BSA ID with admin approval.

### Parent Youth Connection
A join record connecting a parent's **Profile** to a youth's **Profile**, allowing the parent to sign up or withdraw the youth for **Events**. Has status `pending`, `approved`, or `rejected`. Requires admin approval to activate.

### OTP (One-Time Passcode)
A 6-digit code sent via email to verify a user's identity during **Registration**. Stored in `otp_codes` with an expiry timestamp (30 minutes) and a `used` flag. Requesting a new OTP invalidates any existing unused OTP for the same email. Rate-limited to 5 requests per hour per email (counted by existing OTP records in that window). Expired OTP codes are cleaned up by a background goroutine that runs every 24 hours. After the OTP is verified and marked used, the user proceeds to password creation. The OTP email includes a link to `/register/verify?otp_id=<uuid>` so the user can navigate directly from their email.

### Scoutbook Session
An encrypted record of a Bearer JWT token obtained from Scoutbook, stored so the app can call the Scoutbook API on behalf of an admin. Includes the `personGuid`, `expires_at` timestamp, and the encrypted token.

### Scoutbook Sync
The process of importing roster data from Scoutbook into the app's **Profile** table. An admin pastes their Bearer JWT (obtained from the SPA at `advancements.scouting.org`), and the app calls the Scoutbook API at `api.scouting.org` (`POST /organizations/v2/{orgGuid}/orgAdults` and `POST /organizations/v2/{orgGuid}/orgYouths` with body `{includeRegistrationDetails:true, includeExpired:true}`), deduplicates by **BSA ID**, fetches email via `personprofile`, and upserts local **Profile** records. Profiles that no longer appear in Scoutbook are marked `inactive`.
