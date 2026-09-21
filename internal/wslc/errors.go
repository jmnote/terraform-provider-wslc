package wslc

import "errors"

// ErrNotFound is returned by Client methods that look up a single object
// (e.g. GetContainer) when no object with the requested name or ID exists.
var ErrNotFound = errors.New("wslc: object not found")
