// Package verify validates Cumasach skill artifacts. It exposes three entry
// points, all of which return a [Result]:
//
//   - [VerifyPackage] validates a local package archive on disk (package mode).
//   - [VerifyReference] fetches an OCI artifact by reference, then validates it.
//   - [VerifyFetchedArtifact] validates already-fetched OCI bytes, avoiding a
//     redundant registry round-trip for callers that have the artifact in hand.
//
// [VerifyReference] and [VerifyFetchedArtifact] validate structure (config blob
// equals the mirrored manifest, package archive layout) before delegating trust
// to [VerifyPublishedArtifactTrust], which is gated by [TrustPolicy].
package verify

type Result struct {
	Mode                string
	Name                string
	Version             string
	Reference           string
	VerifiedFilesSHA256 bool
}
