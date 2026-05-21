# Milestone 1: Event Management MVP

This document defines the deliverables and success criteria for the first milestone of the scout troop event organization app.

## Scope

The goal of Milestone 1 (M1) is to provide a functional foundation for creating, viewing, and signing up for events, restricted to authorized users.

### Features

1.  **User Authentication & Authorization (RBAC)**
    *   Basic login screen for existing users.
    *   Role-based access control (RBAC) with support for multiple roles (Admin, Scoutmaster, Asst Scoutmaster, Scout, Parent).
    *   Permissions mapped to roles (e.g., only Leaders/Admins can create events).

2.  **Event Management**
    *   Create events with title, description, location, timing, and cost.
    *   Events automatically transition from "Active" to "Past" based on time.

3.  **Event List View**
    *   Chronological list of the next 10 upcoming events.
    *   HTMX-powered lazy loading for more future events.
    *   Reciprocal lazy loading for past events.

4.  **Event Detail & Attendance**
    *   Detailed view of a single event.
    *   Attendee list with distinct styling for the logged-in user.
    *   Self-sign-up and withdrawal functionality for authenticated users.

## Technical Requirements

*   **Storage**: CockroachDB (abstracted behind interfaces).
*   **UI**: HTMX + Go HTML Templates.
*   **Code Quality**: Core logic tested with unit tests.

## Definition of Done

*   [ ] Database schema implemented in CockroachDB.
*   [ ] RBAC system enforces permissions correctly.
*   [ ] Users can sign up and withdraw from events.
*   [ ] Event list correctly handles chronological sorting and pagination.
*   [ ] All Milestone 1 issues in `bd` are closed.
