package credentials

import "errors"

var ErrNotFound = errors.New("credential not found")
var ErrStoreUnavailable = errors.New("credential store is not available on this operating system")
