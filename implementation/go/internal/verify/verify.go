package verify

type Result struct {
	Mode                string
	Name                string
	Version             string
	Reference           string
	VerifiedFilesSHA256 bool
}
