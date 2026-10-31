# Browser regressions

## Purpose

Test the committed dashboard and ceremony assets against a disposable real server.

## Ownership

Private Playwright tooling, owned server/data fixtures and desktop/phone scenarios.

## Local Contracts

- Build the current binary once; give each test its own data directory and random administrator password.
- Use only the owned server and local replication directory. Keep normal dependency/browser caches.
- The server's existing listener binds all interfaces; choose an ephemeral port and use loopback clients with disposable credentials. Never point these tests at an installed service.
- Exercise the real WASM ceremony; imported requests contain only public key/topology fields. Generated cards are disposable test material, never production custody.
- Keep retries disabled and assertions about actual responses, persisted state and refusal behavior. A browser run does not establish a product restore, human acceptance or complete coverage.

## Work Guidance

Use semantic locators when the current markup supplies them; stable IDs cover unlabeled legacy inputs.
Browser tests qualify existing behavior without changing the product's documented security exceptions.
`suite-ui.spec.mjs` checks one navigation region, desktop account placement, active navigation visibility after resizing, and phone replication-footer geometry. It also checks native dialog focus containment, Escape/Cancel focus return (including asynchronous SSO from the Account page), and one-line 40px footer actions. It also checks the shared sign-in card and that the licence link opens the About dialog with the version and licence and returns focus. The existing WASM ceremony test checks Overview priority before and after real key import.

- Empty Restore visibility is checked before pairing and after revoke/clear; the real pairing flow proves configured controls remain available.
- Browser cases verify the retention policy disclosure, required endpoint labels for every preset, and escaped hostile ledger text in desktop/phone layouts.

## Verification

`npm ci && npx playwright install chromium && npm test` from this directory.

## Child DOX Index

None.
