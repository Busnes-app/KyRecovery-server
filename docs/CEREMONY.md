# Ceremony

[Product overview](../README.md) · [Standalone installation](../INSTALL.md)

Commands and unlinked file paths below are relative to the repository root.

## Ceremony: creating the suite recovery keypair

The suite has one long-lived recovery keypair. It is generated once, in the
operator's browser, by a WebAssembly build of `ky-primitives/recoverykey` that is
compiled from this repository and served over an admin session.

1. Sign in as an admin and open `/admin/ceremony`.
2. Choose k (threshold) and n (cards) and generate. The private key is split into
   n printable custodian shares in the tab; **only** `{public_key, threshold,
   total_shares}` is posted to `POST /api/recovery-key`.
3. Print the cards, hand them to the custodians, confirm the import, close the tab.

Rules for the ceremony tab — the seed lives in the tab's memory from generate
until the tab closes and cannot be erased before that:

- A fresh private window, with no browser extensions.
- On a machine that will not hibernate during the ceremony.
- Close the tab once the cards are printed and the import is confirmed.

**Do a print-preview dry run first.** Run the page once with throwaway values
(k=2, n=2), open the print preview from *Print cards*, and confirm on the preview
that the text is black on white, that every share string is fully visible and
wrapped rather than cut off at the right edge, that a card is not split across a
page break, and that no buttons, form or import status appear on paper. Discard
that keypair — do not import it — then run the real ceremony.

The import is single-shot: a second `POST /api/recovery-key` is refused with 409
naming the key already stored. Rotation is a separate procedure that does not
exist yet.
