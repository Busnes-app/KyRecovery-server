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
`suite-ui.spec.mjs` checks one navigation region, desktop account placement, active navigation visibility after resizing, and accessible phone replication-footer geometry.

## Verification

`npm ci && npx playwright install chromium && npm test` from this directory.

## Child DOX Index

None.
