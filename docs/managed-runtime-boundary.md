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

Managed admin access must use named user sessions: static `admin.access_token`
credentials are rejected so MFA, session expiry, and audit attribution cannot
be bypassed. Managed deployments also require an HTTPS `base_url`, a non-local
admin surface, explicit idle and maximum session lifetimes, and disabled pprof
and server debug routes. Plugin and configuration changes remain subject to
the normal admin capability and audit controls. Raw YAML configuration is not
served to managed administrators because it can contain runtime credentials;
the structured settings surface remains available for supported site settings.

Provider-specific provisioning, billing, tenancy, and callback persistence
belong outside this repository. A managed deployment may supply the generic
configuration through its runtime overlay, but this CMS does not contain
provider URLs, tenant IDs, cloud credentials, or Cloud-only package imports.
