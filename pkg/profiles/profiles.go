package profiles

import (
	"os"
	"sort"
	"strings"
)

const ProfilesDir = "profiles"

// GetAvailableProfiles reads the profiles directory and returns the list of available profiles
func GetAvailableProfiles() ([]string, error) {
	var profiles []string

	entries, err := os.ReadDir(ProfilesDir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			profiles = append(profiles, entry.Name())
		}
	}

	// Sort profiles for consistent ordering
	sort.Strings(profiles)

	return profiles, nil
}

// IsValidProfile checks if the given profile name is valid
func IsValidProfile(profile string) bool {
	availableProfiles, err := GetAvailableProfiles()
	if err != nil {
		return false
	}

	for _, validProfile := range availableProfiles {
		if profile == validProfile {
			return true
		}
	}
	return false
}

// GetProfilesString returns a comma-separated string of available profiles
func GetProfilesString() string {
	availableProfiles, err := GetAvailableProfiles()
	if err != nil {
		return "error reading profiles"
	}

	return strings.Join(availableProfiles, ", ")
}

// AvailableProfiles is a convenience variable for backward compatibility
// It returns the current available profiles, but should be used carefully
// as it may return an error state
var AvailableProfiles []string

func init() {
	profiles, err := GetAvailableProfiles()
	if err == nil {
		AvailableProfiles = profiles
	}
}
