# Device Security v1 Brief

Create a cohesive addition to the existing Dipole design system for authenticated
device-session management. Reuse the existing light canvas, deep rail, green
accent, Manrope/Noto Sans SC typography, spacing, and rounded cards.

Required named frames:

- `Device/Desktop/Sessions`: desktop security center with the current session,
  a compact list of other active devices, a low-emphasis refresh action, and a
  dangerous `sign out all other devices` confirmation entry.
- `Device/Mobile/Sessions`: mobile single-column version with the same current
  session and device cards.
- `Device/State Matrix`: loading, empty, unavailable, and sign-out-confirming
  states. A confirmation state must make the affected session explicit.

Add reusable components named `Device Session Row`, `Device Trust Status`, and
`Session Sign-out Confirmation`.

Privacy and product rules:

- Display only device label, coarse browser/device description, last activity,
  and a `current session` state. Do not display IP addresses, node IDs, raw
  connection IDs, user IDs, tokens, precise location, or User-Agent strings.
- This is a security-management view, not a device fingerprinting dashboard.
- Sign-out is destructive and requires an explicit confirmation state. Do not
  create device rename, trust, location, or settings controls.
- Use Chinese product copy consistent with the existing Dipole designs.
