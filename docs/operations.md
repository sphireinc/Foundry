# Production health and operations

This runbook applies to a live `foundry serve` process (including standalone,
service, and Docker modes). Static `foundry build` output has no running health
or metrics endpoints: monitor the CDN or web server that hosts it separately.
Run commands from the project directory with the same environment and config
overlays used by the server. Start with `foundry doctor` before deployment.

## Health checks

`GET /__health` returns JSON with `status`, `version`, `commit`, `managed`,
`instance_id` when present, `admin_ready`, `generated_at`, and `checks`.
`HEAD /__health` performs the same checks and returns no body. Other methods
return 405 with `Allow: GET, HEAD`. Responses have `Cache-Control: no-store`.

- 200 means all checks pass (`status: healthy`).
- 503 means at least one check failed (`status: degraded`). Inspect `checks`.
- Connection refusal or timeout means the probe cannot reach the process.

The checks cover admin configuration readiness and read/write access to
`content`, `data`, `public`, `themes`, and `plugins`. Storage checks create,
write, and remove a temporary file in each directory. They expose logical names
such as `storage.data` and generic errors rather than filesystem paths.
`admin_ready` means admin is enabled and its path is configured; it does not
verify a login, external dependencies, plugin behavior, or rendering. Disabling
admin currently makes health degraded. This is a readiness probe, not a separate
process-only liveness endpoint. Avoid aggressive restart loops on storage faults;
alert and investigate mounts, permissions, capacity, and configuration first.

```sh
curl --fail --max-time 5 http://127.0.0.1:8080/__health
curl --head --max-time 5 http://127.0.0.1:8080/__health
```

Probe every 30 seconds with a 5-second timeout and allow startup time. The
production Compose file uses this endpoint. Test a representative public page
and authenticated admin login separately. The endpoint is unauthenticated and
includes version and instance metadata; restrict it at your reverse proxy if
that metadata should remain private. Health traffic follows the same server
routing as other requests.

## Metrics export

Set a strong random `FOUNDRY_METRICS_TOKEN` in the **server process environment**
and restart to enable `GET /__metrics`. An unset or whitespace-only token keeps
the reserved endpoint at 404 and disables request instrumentation. Supply
`Authorization: Bearer <token>`; missing or incorrect credentials return 401.
Authenticated `HEAD` returns headers only; other methods return 405. Metrics
responses use Prometheus text exposition format 0.0.4 and `Cache-Control: no-store`.
No plugin or extra Go dependency is required.

```sh
# Inject the token through your service's protected environment file or secrets system.
curl --fail --max-time 5 \
  -H "Authorization: Bearer $FOUNDRY_METRICS_TOKEN" \
  http://127.0.0.1:8080/__metrics
```

| Metric                                            | Type    | Meaning                                                                                 |
| ------------------------------------------------- | ------- | --------------------------------------------------------------------------------------- |
| `foundry_http_requests_total{status_class="2xx"}` | Counter | Completed requests, with fixed 1xx–5xx status classes and `other` for nonstandard codes |
| `foundry_http_request_duration_seconds_total`     | Counter | Sum of elapsed request time in seconds                                                  |
| `foundry_http_requests_in_flight`                 | Gauge   | Requests currently being handled                                                        |
| `foundry_go_goroutines`                           | Gauge   | Current Go goroutines                                                                   |
| `foundry_go_heap_alloc_bytes`                     | Gauge   | Allocated Go heap bytes                                                                 |
| `foundry_go_gc_cycles_total`                      | Counter | Completed Go garbage collections                                                        |

HTTP metrics include public pages, assets, health, and mounted admin/plugin
routes. Metrics scrapes (including failed authentication) are excluded.
Counters reset on process restart. Duration includes streaming connections and
is recorded when a request finishes; it is an aggregate, not a latency histogram
or percentile. Metrics contain no URL, query, username, token, or filesystem
labels. Runtime memory collection happens only on an authenticated GET scrape.

Example Prometheus scrape configuration:

```yaml
scrape_configs:
  - job_name: foundry
    metrics_path: /__metrics
    scrape_interval: 30s
    scrape_timeout: 5s
    authorization:
      type: Bearer
      credentials_file: /etc/prometheus/secrets/foundry-token
    static_configs:
      - targets: ['127.0.0.1:8080']
```

Use the address reachable from Prometheus (a container's loopback is its own).
Keep the token file readable only by the scraper account. Use TLS when scraping
across hosts and restrict access at the proxy/firewall. Rotate the token and
restart Foundry, then update the scraper's secret. For Compose, set the token in
the deployment environment; `docker-compose.prod.yml` forwards it to Foundry.

Alert on sustained scrape failures (`up{job="foundry"} == 0`), readiness failures,
and elevated `rate(foundry_http_requests_total{status_class="5xx"}[5m])`.
Average completed-request duration can be calculated as the rate of the duration
counter divided by `sum(rate(foundry_http_requests_total[5m]))`; guard against a
zero denominator. Also monitor disk space, inode availability, backup age, and
host/container resources through your infrastructure monitoring.

## Structured logging

`FOUNDRY_LOG` accepts `debug`, `info` (default), `warn`/`warning`, and `error`.
`FOUNDRY_LOG_FORMAT=json` selects newline-delimited JSON for Foundry's `slog`
logs on stderr. The default and unrecognized formats use readable text. JSON
entries include `time`, `level`, `msg`, and event-specific typed attributes.
Configure these variables before starting the process; changes require restart.
CLI progress output and third-party output can still be plain text.

```sh
FOUNDRY_LOG=info FOUNDRY_LOG_FORMAT=json foundry serve
```

The production Compose file defaults to JSON and `info`:

```sh
docker compose -f docker-compose.prod.yml logs --follow --tail 100 foundry
```

Send stderr to your collector or service log file. Set retention and rotation
at the service/container logging layer and alert on repeated errors. Foundry's
installed services write `.foundry/run/service.log`; rotate it with a strategy
compatible with the manager's open file handles. `foundry logs -f` is the
standalone log viewer. JSON formatting does not redact event attributes.
Avoid debug mode in normal production: verbose request diagnostics can include
queries, remote addresses, user agents, and filesystem context. Restrict access
to logs and apply collector redaction where needed.

## Backup and restore drills

Built-in ZIP and Git snapshots cover the **content tree**, including its config
and media. They do not provide a complete machine recovery image. Independently
back up `data/` (admin accounts, sessions, audit/runtime state), `themes/`,
`plugins/`, external secret material, service definitions, and the executable
version. `public/` can be rebuilt; include it if your recovery-time target requires
it. Treat backups containing configuration and accounts as sensitive.

```sh
foundry backup create
foundry backup list
foundry backup git-snapshot "before deployment"
foundry backup git-log 10
```

Managed ZIPs default to `.foundry/backups`; the production Docker overlay uses
`data/backups` on the persistent data volume. `backup.on_change` is debounced
and is not a scheduled complete-system backup. Retention can prune local ZIPs.
Copy verified archives off the host and keep an independent retention policy.
Never rely on a backup stored only on the disk being protected. Quiesce writers
for consistent content snapshots and stop the service during full filesystem
snapshots, especially when copying account/session data.

Perform this drill on an isolated recovery project, never the live content tree:

1. Record the application revision, config overlays, backup timestamp, archive
   checksum (`shasum -a 256 snapshot.zip`), and expected recovery point/time.
   Copy the archive off-host and verify its checksum after retrieval.
2. Prepare a disposable project with the same executable, themes, plugins, and
   directory names. Restore independently saved data and secrets where needed;
   disable managed callbacks, remote Git pushes, and other production integrations.
   Use a distinct listener port and local-only admin access.
3. Place the ZIP in that project's configured `backup.dir`. Check archive
   integrity with `unzip -t snapshot.zip`. Start the recovery server and sign in
   to its admin UI. Use the Platform → Operations backup controls to restore the selected ZIP.
   There is currently no `foundry backup restore` CLI command.
4. The restore snapshots the existing recovery content first, extracts the
   archive, and replaces the content tree. It requires backup space as well as
   temporary extraction space. It restores content only, and can replace config;
   inspect the restored configuration and reapply isolation settings before
   restarting the recovery server.
5. Run `foundry doctor` and `foundry build` in the recovery project with the
   intended overlays. Restart it, inspect `/__health`, verify representative
   pages/media, and test admin login and a disposable content edit. Compare
   document/media counts and selected checksums with the expected recovery point.
6. Record elapsed recovery time, recovered data age, missing files, and corrective
   actions. Retain drill evidence and repeat after storage/auth/plugin changes
   and on a regular schedule suited to your recovery targets.

For actual recovery, stop traffic and writers, retain the damaged state for
rollback/analysis, restore the verified complete backup set, rebuild, and run
these checks before reopening traffic. A green health response alone does not
prove content or account recovery.

## Service-manager examples

Run the built-in commands from the project root as the deployment user:

```sh
foundry service install
foundry service status
foundry service restart
foundry service stop
foundry service start
```

Linux installation creates a user systemd unit in `~/.config/systemd/user/`;
macOS creates a LaunchAgent in `~/Library/LaunchAgents/`. Names and paths are
printed by installation/status and stored in `.foundry/run/service.json`.
Installation enables and starts the service. Linux users may need
`loginctl enable-linger "$USER"` for logout/reboot persistence. A macOS LaunchAgent
runs in the user's login session; use Docker or a separately managed system
service for deployments that must operate without that session.

The generated definitions run `serve` from the project root and do not capture
shell environment variables or command-line overlays. Configure a manager
provided environment explicitly. For Linux, add an override using the actual
unit name from service status:

```sh
systemctl --user edit <installed-unit-name>.service
```

```ini
[Service]
Environment=FOUNDRY_LOG=info
Environment=FOUNDRY_LOG_FORMAT=json
EnvironmentFile=/absolute/path/to/protected/foundry.env
# Optional explicit production overlay; use the executable printed by service status.
ExecStart=
ExecStart=/absolute/path/to/foundry --config-overlay content/config/site.production.yaml serve
```

Store `FOUNDRY_METRICS_TOKEN=...` and required admin secrets in that protected
file, with permissions limited to the service account. Then run:

```sh
systemctl --user daemon-reload
systemctl --user restart <installed-unit-name>.service
systemctl --user status <installed-unit-name>.service
```

For macOS, add an `EnvironmentVariables` dictionary to the installed plist,
containing `FOUNDRY_LOG`, `FOUNDRY_LOG_FORMAT`, and required secrets. Restrict the
plist's permissions, and unload/reload the agent using its installed path:

```sh
launchctl bootout "gui/$(id -u)" /absolute/path/to/installed.plist
launchctl bootstrap "gui/$(id -u)" /absolute/path/to/installed.plist
```

Use the plist's `ProgramArguments` array to add any required config overlay
before `serve`. Reinstalling can regenerate the plist; preserve your settings.
Check service status, health, metrics authentication, and log collection after
every restart and host reboot. Test these manager-specific steps on your host
before relying on unattended operation.
