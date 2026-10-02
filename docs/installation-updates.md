# Build identity and updates

`foundry version --json` and Admin → Operations separate the **running build**
from the latest GitHub release and the installation method.

- **Tagged release:** an exact tag recorded by the release build, or a tagged Go
  module version. This is build provenance, not signature verification.
- **Source snapshot:** a revision without an exact release tag. The embedded
  release version is only a baseline; it does not mean the snapshot is that release.
- **Modified build:** Go build metadata or the active source checkout records local
  changes. Release replacement could discard custom behavior.
- **Unknown:** insufficient provenance, including older binaries containing only
  the repository's fallback version. Reinstall with current build tooling to
  record provenance; do not assume the fallback establishes release identity.

Containers are an installation method, independent of these build categories.
The image's embedded tag, commit, date and target platform appear in version output;
its image tag is separate from the source release tag. A mounted site's Git history
is not the history of the running container binary. Foundry does not discover the
registry digest from inside the container: inspect it with deployment tooling.

| Installation                          | Update path                                                                                                                                                        |
| ------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Source checkout                       | Preserve local changes, fetch/select the desired revision or tag, rebuild and restart.                                                                             |
| Container                             | Select the runtime or static-builder image and its tag/digest, pull and recreate through deployment tooling. Rebuild locally built images from the desired source. |
| Release binary / Go install           | Download and verify the matching release archive, or reinstall the desired Go module version; replace the executable and restart.                                  |
| Standalone managed tagged release     | Apply Update is available only for a newer release with a matching platform asset.                                                                                 |
| Custom or unverified standalone build | Rebuild/reinstall using its original method and restart; automatic replacement is disabled.                                                                        |

Release comparison is enabled only for unmodified tagged builds. A snapshot,
modified build or unknown build is not labeled “already on the latest release”
just because its baseline matches the latest release. Latest-release information
and installation-specific instructions remain available.

For custom builds, use `scripts/build-release.go` to record version, commit and
build time; it records a release tag only when HEAD has an exact tag. Docker builds
accept `FOUNDRY_BUILD_VERSION`, `FOUNDRY_BUILD_COMMIT`, `FOUNDRY_BUILD_DATE` and
`FOUNDRY_BUILD_TAG`; set the last only to the exact source release tag. The image
workflow supplies it from Git, independently of the published image tag.
