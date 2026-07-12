package rekor

type rekorError struct {
	Code    int64  `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

type rekorProposedEntry interface {
	// Actually the code, currently, accepts anything that can be marshaled into JSON; use at least the Kind marker from
	// shared between RekorHashedrekord / and other accepted formats for minimal sanity checking (but without hard-coding
	// RekorHashedRekord in particular).

	Kind() string
	SetKind(string)
}

type rekorLogEntryAnon struct {
	Attestation    *rekorLogEntryAnonAttestation  `json:"attestation,omitempty"`
	Body           any                            `json:"body"`
	IntegratedTime *int64                         `json:"integratedTime"`
	LogID          *string                        `json:"logID"`
	LogIndex       *int64                         `json:"logIndex"`
	Verification   *rekorLogEntryAnonVerification `json:"verification,omitempty"`
}

type rekorLogEntryAnonAttestation struct {
	Data []byte `json:"data,omitempty"`
}

type rekorLogEntryAnonVerification struct {
	InclusionProof       *rekorInclusionProof `json:"inclusionProof,omitempty"`
	SignedEntryTimestamp []byte               `json:"signedEntryTimestamp,omitempty"`
}

type rekorLogEntry map[string]rekorLogEntryAnon

type rekorInclusionProof struct {
	Checkpoint *string  `json:"checkpoint"`
	Hashes     []string `json:"hashes"`
	LogIndex   *int64   `json:"logIndex"`
	RootHash   *string  `json:"rootHash"`
	TreeSize   *int64   `json:"treeSize"`
}
