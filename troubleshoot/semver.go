package troubleshoot

import (
	"fmt"
	"strconv"
	"strings"
)

// A SemVer is a semantic version string.
type SemVer struct {
	version [3]byte
	suffix  string
}

// String returns the string representation of the semantic version.
func (v SemVer) String() string {
	if v.suffix != "" {
		return fmt.Sprintf("v%d.%d.%d-%s", v.version[0], v.version[1], v.version[2], v.suffix)
	}
	return fmt.Sprintf("v%d.%d.%d", v.version[0], v.version[1], v.version[2])
}

// Suffix returns the suffix of the semantic version.
func (v SemVer) Suffix() string {
	return v.suffix
}

// Cmp compares two semantic versions.
// Returns -1 if a < b, 0 if a == b, 1 if a > b
func (v SemVer) Cmp(b SemVer) int {
	// Compare two semantic versions
	switch {
	case v.version[0] != b.version[0]:
		return int(v.version[0]) - int(b.version[0])
	case v.version[1] != b.version[1]:
		return int(v.version[1]) - int(b.version[1])
	case v.version[2] != b.version[2]:
		return int(v.version[2]) - int(b.version[2])
	case v.suffix == "" && b.suffix != "":
		return 1 // v is a release version, b is a pre-release version
	case v.suffix != "" && b.suffix == "":
		return -1 // v is a pre-release version, b is a release version
	default:
		return 0
	}
}

// UnmarshalText implements encoding.TextUnmarshaler
func (v *SemVer) UnmarshalText(buf []byte) error {
	if len(buf) == 0 {
		return fmt.Errorf("empty version string")
	}
	version := string(buf)
	if version[0] != 'v' {
		return fmt.Errorf("invalid version format: %s", version)
	}

	var suffix string
	version = version[1:] // Remove the leading 'v'
	if suffixPos := strings.Index(version, "-"); suffixPos >= 0 {
		// remove optional suffix
		suffix = version[suffixPos+1:]
		version = version[:suffixPos]
	}

	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return fmt.Errorf("invalid version format: %s", version)
	}
	major, err := strconv.ParseUint(parts[0], 10, 8)
	if err != nil {
		return fmt.Errorf("invalid major version: %s", parts[0])
	}

	minor, err := strconv.ParseUint(parts[1], 10, 8)
	if err != nil {
		return fmt.Errorf("invalid minor version: %s", parts[1])
	}

	patch, err := strconv.ParseUint(parts[2], 10, 8)
	if err != nil {
		return fmt.Errorf("invalid patch version: %s", parts[2])
	}
	v.version = [3]byte{byte(major), byte(minor), byte(patch)}
	v.suffix = suffix
	return nil
}
