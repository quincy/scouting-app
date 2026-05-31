# Context: Scout App

This document defines the ubiquitous language for the scout troop event organization app.

## Glossary

### Event
A planned social or troop activity with a title, description, location, and timing.

### Campout
A specific type of **Event** that typically spans multiple days.

### User
A security principal in the system, identified by their email address and authenticated via password. A User has no PII — all personal information lives on the associated **Profile**. Users have **Roles** that determine their **Permissions**.

### Profile
A 1:1 extension of a **User** containing all PII and personal information (name, birthdate, phone, etc.). Profiles are synced from an external system rather than managed in-app.

### Role
A designation assigned to a **User** that determines their permissions. A **User** can have multiple **Roles**.

### Permission
A specific action that a **User** is allowed to perform (e.g., "Create Event", "Sign up for Event"). Permissions are mapped to **Roles**.

### Attendee
A **User** who has signed up to participate in a specific **Event**. An Attendee has a status (`signed_up`, `canceled`) and may hold one or more **Responsibilities** for that Event.

### Attendee Status
Indicates whether an **Attendee** is currently participating (`signed_up`) or has been removed (`canceled`).

### Responsibility
A designated role an **Attendee** holds for a specific **Event** (e.g., `driver`, `cook`). An Attendee may hold multiple responsibilities.

### Event Type
A classification of an **Event** (e.g., `campout`). Defined as a fixed set of known values.

### Sign-up
The action of a **User** registering themselves as an **Attendee** for an **Event**.

### Withdraw
The action of a **User** removing themselves from the **Attendee** list.

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
