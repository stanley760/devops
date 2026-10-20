package iam

import (
	"encoding/json"
	"strconv"
	"strings"
)

// JSON encoding/decoding of storage-layer ID arrays.
//
// This is an immutable storage contract (existing data + unique index + SQL numeric matching
// semantics all depend on it):
//
//	valid form: [4,5]          numeric array
//	dirty data:  ["4","5"]     string elements → treated as dirty data; a whole-parse failure returns an empty set
//
// Why []types.ID cannot be marshaled directly: types.ID is an API contract type whose
// MarshalJSON emits string elements (["4","5"] — a frontend contract requirement that IDs be
// stringified to avoid JS 2^53 precision loss). Reusing it directly would leak the API form
// into the storage layer, inconsistent with the seed data and the numeric matching semantics
// of JSON_CONTAINS(post_ids, 4).
//
// Before the refactor, this logic lived in dao/user_admin.go:209-241 (PostIDsOf / PostIDsJSON),
// and only because dao had a reverse dependency on internal/types did it have to hand-write
// this conversion. Once the domain layer no longer knows the API contract type, the
// encoding/decoding naturally belongs here.

// EncodeIDArray converts a numeric ID slice to a storage JSON string. An empty set returns "" (the repository decides whether to write NULL).
func EncodeIDArray(ids []int64) string {
	if len(ids) == 0 {
		return ""
	}
	// copy and strip nil semantics: an empty slice marshals to [] rather than null.
	b, err := json.Marshal(ids)
	if err != nil {
		return ""
	}
	return string(b)
}

// DecodeIDArray converts a storage JSON string to a numeric ID slice.
//
// Tolerance semantics (verbatim aligned with the existing implementation):
//   - empty string → empty slice (not nil)
//   - invalid JSON or non-numeric elements → empty slice, with no compatibility read
func DecodeIDArray(raw string) []int64 {
	if raw == "" {
		return []int64{}
	}
	var ids []int64
	if err := json.Unmarshal([]byte(raw), &ids); err != nil {
		return []int64{}
	}
	if ids == nil {
		return []int64{}
	}
	return ids
}

// decodeInt64JSONArray is a lenient variant of DecodeIDArray: invalid input returns nil rather than an
// empty slice, for judgment scenarios like "custom data-permission dept sets" where invalid means unconfigured.
func decodeInt64JSONArray(raw string) []int64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var ids []int64
	if err := json.Unmarshal([]byte(raw), &ids); err != nil {
		return nil
	}
	return ids
}

// NormalizeIDArrayJSON normalizes to the numeric array form (unified semantics before persistence).
//
// Before the refactor, this normalization was handled by devops-infra/datascope.NormalizeDeptIDs
// for role dept sets; here it is an isomorphic implementation so post sets (post_ids) can
// reuse the same semantics.
func NormalizeIDArrayJSON(raw string) string {
	return EncodeIDArray(DecodeIDArray(raw))
}

// IDsFromStrings converts a list of numeric strings to an int64 slice (invalid entries are skipped).
func IDsFromStrings(raw []string) []int64 {
	out := make([]int64, 0, len(raw))
	for _, s := range raw {
		v, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
		if err != nil {
			continue
		}
		out = append(out, v)
	}
	return out
}
