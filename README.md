# Scout App

A web application for managing scout unit events. Built with Go, HTMX, and CockroachDB.

Integrates with the Scoutbook API for roster sync, supports event management, attendee sign-up, role-based access
control, and family connections for parent/youth account linking.

## Features

- **Event Management** — Create, edit, and manage unit events with attendee sign-up, responsibilities (drivers, SPL,
  coordinator), and seatbelt capacity tracking.
- **Scoutbook Sync** — Imports and reconciles unit roster from the Scoutbook API with position-based role assignment.
- **Registration & Authentication** — Email-based OTP registration flow with password authentication and session
  management.
- **Role-Based Access Control** — Fine-grained permissions mapped to roles, with admin management interface.
- **Admin Tools** — Roster management, family connection approvals, role administration, and profile status controls.
- **Event Communication** — Send announcements and reminders to attendees or the full unit via email.

## License

[MIT](LICENSE)

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines on how to contribute.

## Development

See [DEVELOPING.md](DEVELOPING.md) for setup, build, test, and run instructions.
