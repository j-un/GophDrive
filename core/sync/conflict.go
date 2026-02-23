package sync

// CheckConflict compares local and remote ETag values.
// Returns true if they are different (conflict exists).
func CheckConflict(localEtag, remoteEtag string) bool {
	return localEtag != remoteEtag
}
