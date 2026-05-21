# Context: Scout Events App

## Glossary

### Media
- **Media Store**: Google Drive is the primary destination for all user-uploaded photos and videos.
- **Upload Flow**: Users upload media through the application, which then pushes the files to a specific, pre-configured Google Drive folder designated for the event.
- **Legacy Archive**: Existing troop photos currently in Google Drive may be linked to events within the application.

### Finance
- **Payment Provider**: Stripe (primary) or Square (secondary) are used for all external transactions.
- **Financial Data**: The application does not maintain an internal ledger. It fetches transaction, refund, and payout data directly from the payment provider APIs as needed for reporting.
- **Patrol Reimbursement**: Calculated by the app but recorded externally in the troop's primary accounting system or via the payment provider.

### Membership
- **Youth**: A scout participant in the troop.
- **Guardian**: An adult (parent or authorized volunteer) linked to one or more Youth members.
- **Scoutbook**: The external absolute source of truth for the troop's roster and relationships.
- **Strict Sync**: Roster imports from Scoutbook automatically overwrite existing user data and guardianship links. Missing records in the import result in deactivation/soft-deletion in the app.
- **Claiming**: The process where a user verifies their identity via a single-use token to set up their account password.
