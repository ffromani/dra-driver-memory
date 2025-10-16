package v0

import "errors"

// ValidateHugePageSize returns the internal (sysfs) hugepage size to use
// and nil error if is a supported size; otherwise returns empty string
// and an error detailing the reason
func ValidateHugePageSize(hps HugePageSize) (string, error) {
	hpSize := string(hps) // shortcut
	if hpSize == "1G" || hpSize == "1Gi" || hpSize == "1g" {
		return "1048576kB", nil
	}
	if hpSize == "2M" || hpSize == "2Mi" || hpSize == "2m" {
		return "2048kB", nil
	}
	return "", errors.New("unsupported size")
}
