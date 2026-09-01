# Contributing

Decisions that shape the library are recorded as ADRs in `docs/adr`. Read the
relevant one before reopening a question it settles.

## The description schema

`desc` defines the wire format a program publishes to describe its own command
line. The JSON Schema for that format lives at `docs/desc.schema.json`, and is
published at

    https://static.hotsrc.dev/climux/schema/v1.json

which is the `$id` a document's consumer resolves. Those are two copies of one
document: the repository holds the source of truth, and the published copy is
uploaded by hand.

Nothing detects drift between them. No test reaches the network, so a schema
change that is committed but never published leaves consumers resolving a
stale document while the suite stays green.

Only the maintainer can publish: uploading needs credentials nobody else
holds. So a change that touches `docs/desc.schema.json` is not finished when
it merges, and the contributor's part is to say so in the pull request. Call
it out there and the upload follows.

Within `schemaVersion` 1 the format only gains keys, and a consumer must
ignore keys it does not know. Removing or renaming one is a wire break and
needs a new version. `TestWireKeyPaths` in `desc` fails on either and says
which happened.
