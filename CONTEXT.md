# Context: Scout App

This document defines the ubiquitous language for the scout troop event organization app.

## Glossary

### Event
A planned social or troop activity with a title, description, location, and timing.

### Campout
A specific type of **Event** that typically spans multiple days.

### User
A person registered in the system. Users have roles that determine their permissions.

### Role
A designation assigned to a **User** that determines their permissions. A **User** can have multiple **Roles**.

### Permission
A specific action that a **User** is allowed to perform (e.g., "Create Event", "Sign up for Event"). Permissions are mapped to **Roles**.

### Attendee
A **User** who has signed up to participate in a specific **Event**.

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
