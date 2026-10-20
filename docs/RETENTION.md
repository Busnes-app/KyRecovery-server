# Retention

[Product overview](../README.md) · [Standalone installation](../INSTALL.md)

Commands and unlinked file paths below are relative to the repository root.

## Backup retention

In **Capsules → Backup retention**, admins can keep all backups for a recent
window, plus the newest backup per UTC ISO week and calendar month for each
paired product. For example, set **14 days / 8 weeks / 12 months**. Each window
counts back from today; the windows overlap rather than adding together. Age
uses the server's deposit time. Days **0** keeps everything forever (the default);
weeks or months **0** disables that tier.

Save the policy, review the expired count and bytes, then choose **Purge expired**
and confirm. This applies to existing backups too. Purge is manual and can remove
a product's last backup once it falls outside every window. It deletes local
capsule files and catalog rows, preserving audit and replication history.
Offsite copies require separate lifecycle policies at their destinations.

`GET /api/retention` returns the saved policy and purge preview. Admins save with
`POST /api/retention` and purge with `POST /api/retention/purge`, both using
`{"days":14,"weeks":8,"months":12}`. Purge rejects a policy that differs from the
saved one. Failures report `purged_count`; after repairing storage or audit health,
retry to clean up remaining expired backups. An unavailable audit ledger prevents
purging.
