# Managed Runtime Boundary

Foundry remains a provider-neutral CMS. Managed mode is optional and disabled
unless `foundry.managed.enabled` is explicitly set in site configuration.
Standalone builds, serve mode, plugins, themes, and admin behavior do not
require a Cloud account or control-plane dependency.

Managed integrations are generic runtime features: signed registration and
status callbacks, bootstrap application, health reporting, and managed backup
snapshots. They require an explicit instance ID, HTTPS callback URL, strong
shared secret, and secure admin configuration. Callback code is skipped when
managed mode is disabled.

Provider-specific provisioning, billing, tenancy, and callback persistence
belong outside this repository. A managed deployment may supply the generic
configuration through its runtime overlay, but this CMS does not contain
provider URLs, tenant IDs, cloud credentials, or Cloud-only package imports.
