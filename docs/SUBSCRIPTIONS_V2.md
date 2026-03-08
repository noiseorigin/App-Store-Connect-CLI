# Subscriptions V2 Taxonomy

This note captures the canonical v2 subscriptions command matrix and the
migration mapping from the older flat `asc subscriptions` surface.

## Canonical Families

- `asc subscriptions groups ...`
- `asc subscriptions list|get|create|update|delete ...`
- `asc subscriptions pricing summary ...`
- `asc subscriptions pricing prices ...`
- `asc subscriptions pricing price-points ...`
- `asc subscriptions pricing availability ...`
- `asc subscriptions offers introductory ...`
- `asc subscriptions offers promotional ...`
- `asc subscriptions offers offer-codes ...`
- `asc subscriptions offers win-back ...`
- `asc subscriptions review screenshots ...`
- `asc subscriptions review app-store-screenshot ...`
- `asc subscriptions review submit ...`
- `asc subscriptions review submit-group ...`
- `asc subscriptions promoted-purchases ...`
- `asc subscriptions localizations ...`
- `asc subscriptions images ...`
- `asc subscriptions grace-periods ...`

## Old To New Mapping

| Old path | Canonical v2 path | Compatibility behavior |
| --- | --- | --- |
| `asc subscriptions pricing` | `asc subscriptions pricing summary` | Legacy shortcut stays runnable and warns on stderr |
| `asc subscriptions prices ...` | `asc subscriptions pricing prices ...` | Hidden deprecated shim |
| `asc subscriptions price-points ...` | `asc subscriptions pricing price-points ...` | Hidden deprecated shim |
| `asc subscriptions availability ...` | `asc subscriptions pricing availability ...` | Hidden deprecated shim |
| `asc subscriptions introductory-offers ...` | `asc subscriptions offers introductory ...` | Hidden deprecated shim |
| `asc subscriptions promotional-offers ...` | `asc subscriptions offers promotional ...` | Hidden deprecated shim |
| `asc subscriptions offer-codes ...` | `asc subscriptions offers offer-codes ...` | Hidden deprecated shim |
| `asc subscriptions win-back-offers ...` | `asc subscriptions offers win-back ...` | Hidden deprecated shim |
| `asc subscriptions review-screenshots ...` | `asc subscriptions review screenshots ...` | Hidden deprecated shim |
| `asc subscriptions app-store-review-screenshot ...` | `asc subscriptions review app-store-screenshot ...` | Hidden deprecated shim |
| `asc subscriptions submit ...` | `asc subscriptions review submit ...` | Hidden deprecated shim |
| `asc subscriptions groups submit ...` | `asc subscriptions review submit-group ...` | Hidden deprecated shim |

## Canonical Flag Direction

Canonical visible paths prefer typed selectors:

- `--group-id`
- `--reference-name`
- `--subscription-id`
- `--offer-code-id`
- `--price-point-id`
- `--screenshot-id`
- `--availability-id`

Older flags stay accepted on compatibility paths during migration where needed:

- `--group`
- `--ref-name`
- generic `--id` where older commands already used it
- `--subscription` on older win-back flows

## Compatibility Rules

- Deprecated paths remain executable.
- Deprecated paths are hidden from primary discovery surfaces when a canonical
  v2 path exists.
- Deprecation warnings go to `stderr` only.
- Canonical wrappers should rewrite runtime error prefixes so user-facing errors
  match the new command path rather than the legacy implementation path.
