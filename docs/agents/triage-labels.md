# Triage Label Vocabulary

The `triage` skill moves issues through a state machine using the following labels.

## Mappings

| Role              | Label             | Description                  |
|-------------------|-------------------|------------------------------|
| `needs-triage`    | `needs-triage`    | Maintainer needs to evaluate |
| `needs-info`      | `needs-info`      | Waiting on reporter          |
| `ready-for-agent` | `ready-for-agent` | Fully specified, AFK-ready   |
| `ready-for-human` | `ready-for-human` | Needs human implementation   |
| `wontfix`         | `wontfix`         | Will not be actioned         |

> Note: Apply labels via `gh api repos/quincy/scouting-app/issues/{number} -X PATCH -f labels[]="label-name"`.
