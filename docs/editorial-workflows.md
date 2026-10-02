# Editorial workflows for teams

Foundry uses its existing `draft`, `in_review`, `scheduled`, `published`, and
`archived` states for team workflows. Assignments, review decisions, and revision
comparisons are built into the admin API and default admin theme.

## Enable approval enforcement

```yaml
editorial:
  require_approval: true
```

This setting defaults to `false`, preserving existing publication workflows.
Enable it before allowing a team to publish through the admin API. Direct edits
to repository files, plugins with filesystem access, and configuration changes
remain trusted operator actions; this is an admin workflow boundary.

With enforcement enabled, an editor must approve the saved revision through an
independent reviewer before publishing or scheduling it. Approval is separate
from publication: reviewers decide; editors publish. Existing published files
remain published when the policy is enabled, but future publication actions
require a current approval. Editing approved or published content clears its
approval and returns it to draft. Restore also returns content to draft and
requires fresh review.

## Assign and review a document

1. Save a draft, then open **Team review** in the document editor.
2. An editor sets its Owner, Assignee, and Reviewer using active admin usernames.
   Empty Assignee or Reviewer clears that assignment; empty Owner preserves it.
3. The owner or assigned author saves the content and selects **Request Review**,
   then saves the document to enter `in_review`.
4. The reviewer loads that saved revision and selects **Approve saved revision**
   or **Request changes**, optionally adding a review comment.
5. An editor selects **Publish approved revision** and saves, or sets a future
   Scheduled Publish time and the `scheduled` state.

Review controls act on saved content. Unsaved changes must be saved first. A
stale revision is rejected instead of approving newer content inadvertently.
The assigned reviewer is the only reviewer who may decide when one is specified;
otherwise any reviewer can decide. The owner, assignee, and last content editor
cannot approve or request changes on their own revision. A reviewer can read
content and make decisions without permission to edit its body or publish it.

Ownership uses the existing document `author` field. Assignment grants an author
access to the assigned document without changing its owner. Raw saves cannot
transfer ownership or forge assignments, approvals, or review history. Use the
assignment action instead. Changes to assignments clear approval; with enforced
review, assignment changes to published or scheduled content return it to draft.

Editorial actions and status changes retain snapshots in the existing revision
history. The document also records decision actor, time, revision, and comment
under its server-managed `editorial` frontmatter. The editor displays recent
review events; complete events remain in the document and retained revisions.

## Capabilities

| Role     | Editorial access                                                        |
| -------- | ----------------------------------------------------------------------- |
| Author   | Read and edit owned or assigned documents; request review               |
| Reviewer | Read documents; approve or request changes                              |
| Editor   | Edit, assign ownership/reviewers, review others' revisions, and publish |
| Admin    | All capabilities; independent review is still required                  |

Custom roles can use `documents.assign`, `documents.review`, and
`documents.publish`. These supplement the existing document read/write and
history capabilities; `.own` grants remain scoped to ownership or assignment.

## Scheduling and revision comparison

A scheduled document becomes eligible for publication when its publish time
arrives and is hidden again after its unpublish time. Static deployments need a
build and deployment at those times; merely setting a timestamp does not launch
a deployment job. Scheduled publication requires a future publish time, and an
unpublish time must follow it.

The existing revision comparison now returns top-level frontmatter changes and
a separate body diff in addition to the original raw comparison. This makes
content changes easier to distinguish from workflow and metadata changes.
Comparisons accept at most 1 MiB and 2,000 lines per revision to bound memory use.

## Admin API and SDK

`POST /__admin/api/documents/editorial` (adjust for your configured admin prefix):

```json
{
  "source_path": "posts/story.md",
  "action": "approve",
  "expected_revision": "revision hash from document.editorial.revision",
  "note": "Fact check complete"
}
```

Actions are `assign`, `approve`, and `request_changes`. Assignment additionally
accepts `owner`, `assignee`, and `reviewer`. Include `lock_token` when required by
your document access. Each request requires an authenticated admin identity.

`GET /api/document` exposes `editorial`, including assignments, approval, events,
the current `revision`, and `require_approval`. The admin SDK exposes mutations as
`documents.editorial(input)`. Existing save, status, restore, and diff endpoints
participate in the same policy; publishing raw frontmatter bypasses no checks.

`POST /api/documents/diff` additionally returns `body_diff` and
`frontmatter_changes` (`field`, `before`, `after`). Its existing `diff`, `left_raw`,
and `right_raw` fields remain available.
