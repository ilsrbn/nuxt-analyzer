# Latest Unresolved PR #3 Review Comment

**Comment ID:** 3208279670
**File:** `internal/analyzer/scanner.go:31`
**Priority:** P2
**Status:** Unresolved

## Issue

When `WalkDir` reports a directory read/stat error that is not permission or not-exist (e.g. an I/O error under an otherwise valid project root), the current code returns `nil`, so scanning continues with that subtree silently omitted and the generated graph can be incomplete without any error.

## Current Code (scanner.go:26-34)

```go
if walkErr != nil {
	if d == nil || d.IsDir() {
		if os.IsPermission(walkErr) || os.IsNotExist(walkErr) {
			return filepath.SkipDir
		}
		return nil // BUG: silently ignores directory errors
	}
	return walkErr
}
```

## Fix

Return `walkErr` for any directory error that is not permission or not-exist:

```go
if walkErr != nil {
	if d == nil || d.IsDir() {
		if os.IsPermission(walkErr) || os.IsNotExist(walkErr) {
			return filepath.SkipDir
		}
	}
	return walkErr
}
```

This keeps the graceful skip for permission/not-exist errors but propagates I/O and other unexpected directory walk errors instead of silently swallowing them.
