# User Status v1

`status.schema.json` is the language-neutral source for values persisted in
`users.status` and exchanged by future User Service clients.

| Symbol | Value | Meaning |
| --- | ---: | --- |
| `normal` | `1` | The account can authenticate and use IM capabilities. |
| `disabled` | `2` | Authentication and authenticated transports reject the account. |

Value `0` is reserved as invalid. Migration v27 maps historical `0` rows to
`normal`, changes the database default to `1`, and enforces the v1 enum.
