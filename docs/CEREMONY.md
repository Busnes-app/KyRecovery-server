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

The first import is single-shot: a second `POST /api/recovery-key` is refused with
409 naming the key already stored. A new key replaces it only through rotation.

## Rotating the recovery key

Rotate when custodians change, a card is lost or exposed, or you want a different k or
n. Rotation is a new ceremony; nothing is derived from the old key, and the server never
holds either private half.

1. Sign in as an admin and open `/admin/ceremony` under the same rules as the first
   ceremony. The page says this is a rotation and names the current key.
2. Choose the new k and n, generate, print the new cards. **Write down the new key ID**:
   it is printed on every card and is what each product will confirm.
3. Import. The page posts `{public_key, threshold, total_shares, replaces_key_id}` to
   `POST /api/recovery-key/rotate`; `replaces_key_id` must be the current key, so a tab
   left open across another rotation is refused with 409. A key the store has ever held,
   retired or not, is also refused: run a new ceremony instead.
4. The new key is now **current**; the old one is **previous**. Both are listed under
   *Custodians → Recovery key* with their k-of-n, the number of retained capsules sealed to
   each, and the products still pinned to the old one.
5. **Re-pair every product.** From now on a deposit sealed to the old key is refused with
   409 ("rotated out; re-pair this product"). For each product: generate a pairing code,
   enter it in the product, and confirm in the product that the key ID it received equals
   the ID on the new cards before it replaces its pin. Then revoke the product's old
   pairing here. Until the product changes listed under *Product follow-up* below ship, a
   product refuses a different key on re-pair; see its documentation for clearing the pin.
6. Keep the old cards. Every capsule sealed to the old key still opens only with k of the
   old cards. The server cannot re-seal it to the new key: that would mean decrypting it.
7. **Retire** the old key once nothing you might restore needs it. Retirement is refused
   while any retained capsule is sealed to the key, and the refusal lists those capsules.
   They leave as retention purges them. Offsite copies are not purged by local retention,
   so check your replicas before you destroy the old cards: a retired key's cards still
   open any copy of its capsules that survives elsewhere.

Rotation and retirement are admin-only, refused from another origin, and recorded in
the audit ledger before they are made (`recovery_key_rotation_requested`,
`recovery_key_retirement_requested`) and after (`recovery_key_rotated`,
`recovery_key_retired`). If the ledger cannot append, the change is refused with 503.
KyRecovery has no step-up re-authentication; an admin session is the bar.

### Why products must confirm the new key

A product pins the recovery key it was handed at pairing, and seals every backup to it.
If the server could swap that key silently, anyone who controlled the server could make
every future backup readable with cards they printed themselves. The recovery key is an
encryption key with no signature scheme, so the old key cannot vouch for the new one.
The person re-pairing each product is the check: they compare the key ID the product
received with the ID on the new cards, which they hold.

### Product follow-up

KyRecovery's side ships here. Each product (KySignOn, KyPassword, KyBookmarks, KyNotes,
KyPost) needs, through `ky-primitives/recoveryclient`:

- a re-pair that replaces the pinned key only after the operator confirms the new key ID
  (today `StoreRecoveryKey` refuses any different key);
- on a deposit 409 whose body carries `"code": "recovery_key_rotated"`, a clear
  "KyRecovery's key was rotated: re-pair" status instead of a generic failure. The body
  also names `sealed_key_id` and `current_key_id`, so the product can show the ID the
  operator should expect on re-pair; the new key itself arrives only through pairing;
- its restore screen to say which key ID a capsule is sealed to (the manifest's
  `recovery_key_id`), so the operator fetches the matching cards.
