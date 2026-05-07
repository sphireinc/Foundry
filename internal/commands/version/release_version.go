package version

// ReleaseVersion is the repo-carried release fallback used when a build does
// not have Git metadata available, such as container builds from a source
// archive or a Docker context without .git.
const ReleaseVersion = "v1.4.4"
