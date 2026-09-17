// Package update defines the verification contract for application updates.
package update

import (
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"unicode"
)

// CurrentVersion is set at build time with -ldflags "-X <package>.CurrentVersion=x.y.z".
// It is the application's only current-version source.
var CurrentVersion = "0.0.0-dev"

// ReleasePublicKey is a base64-encoded Ed25519 public key set by the release build.
// It must never contain private signing material.
var ReleasePublicKey string

// Manifest is the signed, transport-independent description of one AppImage release.
type Manifest struct {
	Version   string `json:"version"`
	URL       string `json:"url"`
	SHA256    string `json:"sha256"`
	Signature string `json:"signature"`
}

// Release is a verified release eligible for download after user approval.
type Release struct {
	Version string
	URL     string
	SHA256  string
}

// SignedBytes returns the deterministic bytes that a release workflow must sign.
func (m Manifest) SignedBytes() ([]byte, error) {
	_, err := parseVersion(m.Version)
	if err != nil {
		return nil, fmt.Errorf("invalid version: %w", err)
	}
	if err := validateAppImageURL(m.URL); err != nil {
		return nil, err
	}
	if err := validateDigest(m.SHA256); err != nil {
		return nil, err
	}
	return []byte("version=" + m.Version + "\nurl=" + m.URL + "\nsha256=" + m.SHA256 + "\n"), nil
}

// VerifyManifest validates and verifies a manifest against the supplied application
// version and base64-encoded Ed25519 public key. It accepts only newer releases.
// It does not make the manifest eligible for replacement; ReplacementEligible
// structurally requires user approval before yielding a Release.
func VerifyManifest(m Manifest, currentVersion, encodedPublicKey string) error {
	current, err := parseVersion(currentVersion)
	if err != nil {
		return fmt.Errorf("invalid current version: %w", err)
	}
	payload, err := m.SignedBytes()
	if err != nil {
		return err
	}
	candidate, err := parseVersion(m.Version)
	if err != nil {
		return fmt.Errorf("invalid version: %w", err)
	}
	if compareParsed(candidate, current) <= 0 {
		return errors.New("release version must be newer than current version")
	}
	publicKey, err := decodePublicKey(encodedPublicKey)
	if err != nil {
		return err
	}
	signature, err := base64.StdEncoding.DecodeString(m.Signature)
	if err != nil || len(signature) != ed25519.SignatureSize {
		return errors.New("invalid manifest signature")
	}
	if !ed25519.Verify(publicKey, payload, signature) {
		return errors.New("invalid manifest signature")
	}
	return nil
}

// ReplacementEligible verifies a release and requires an explicit user approval.
// It deliberately performs no download, file replacement, restart, or process action.
func ReplacementEligible(m Manifest, currentVersion, encodedPublicKey string, userApproved bool) (Release, error) {
	if !userApproved {
		return Release{}, errors.New("user approval is required before replacement or restart")
	}
	if err := VerifyManifest(m, currentVersion, encodedPublicKey); err != nil {
		return Release{}, err
	}
	return Release{Version: m.Version, URL: m.URL, SHA256: m.SHA256}, nil
}

// CompareVersions compares semantic versions. It returns -1, 0, or 1.
func CompareVersions(left, right string) (int, error) {
	leftVersion, err := parseVersion(left)
	if err != nil {
		return 0, fmt.Errorf("invalid left version: %w", err)
	}
	rightVersion, err := parseVersion(right)
	if err != nil {
		return 0, fmt.Errorf("invalid right version: %w", err)
	}
	return compareParsed(leftVersion, rightVersion), nil
}

func decodePublicKey(encoded string) (ed25519.PublicKey, error) {
	if encoded == "" {
		return nil, errors.New("missing release public key")
	}
	key, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(key) != ed25519.PublicKeySize {
		return nil, errors.New("invalid release public key")
	}
	return ed25519.PublicKey(key), nil
}

func validateAppImageURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return errors.New("release URL must be an HTTPS URL")
	}
	if !strings.HasSuffix(parsed.Path, ".AppImage") {
		return errors.New("release URL must point to an AppImage")
	}
	return nil
}

func validateDigest(digest string) error {
	if len(digest) != 64 {
		return errors.New("SHA-256 digest must contain 64 lowercase hexadecimal characters")
	}
	for _, character := range digest {
		if !(character >= '0' && character <= '9' || character >= 'a' && character <= 'f') {
			return errors.New("SHA-256 digest must contain 64 lowercase hexadecimal characters")
		}
	}
	return nil
}

type version struct {
	major, minor, patch uint64
	pre, build          []string
}

func (v version) String() string {
	result := fmt.Sprintf("%d.%d.%d", v.major, v.minor, v.patch)
	if len(v.pre) != 0 {
		result += "-" + strings.Join(v.pre, ".")
	}
	if len(v.build) != 0 {
		result += "+" + strings.Join(v.build, ".")
	}
	return result
}

func parseVersion(raw string) (version, error) {
	if strings.HasPrefix(raw, "v") {
		raw = raw[1:]
	}
	if raw == "" {
		return version{}, errors.New("must be a semantic version")
	}
	partsWithBuild := strings.Split(raw, "+")
	if len(partsWithBuild) > 2 || (len(partsWithBuild) == 2 && partsWithBuild[1] == "") {
		return version{}, errors.New("invalid build metadata")
	}
	parts := strings.SplitN(partsWithBuild[0], "-", 2)
	core := strings.Split(parts[0], ".")
	if len(core) != 3 {
		return version{}, errors.New("must have major.minor.patch components")
	}
	values := make([]uint64, 3)
	for index, component := range core {
		if !validNumericIdentifier(component) {
			return version{}, errors.New("numeric components must not have leading zeroes")
		}
		value, err := strconv.ParseUint(component, 10, 64)
		if err != nil {
			return version{}, err
		}
		values[index] = value
	}
	parsed := version{major: values[0], minor: values[1], patch: values[2]}
	if len(parts) == 2 {
		if parts[1] == "" {
			return version{}, errors.New("pre-release must not be empty")
		}
		parsed.pre = strings.Split(parts[1], ".")
		for _, identifier := range parsed.pre {
			if identifier == "" || !validIdentifier(identifier) || (isDigits(identifier) && !validNumericIdentifier(identifier)) {
				return version{}, errors.New("invalid pre-release identifier")
			}
		}
	}
	if len(partsWithBuild) == 2 {
		parsed.build = strings.Split(partsWithBuild[1], ".")
		for _, identifier := range parsed.build {
			if identifier == "" || !validIdentifier(identifier) {
				return version{}, errors.New("invalid build metadata")
			}
		}
	}
	return parsed, nil
}

func validNumericIdentifier(value string) bool {
	return isDigits(value) && (len(value) == 1 || value[0] != '0')
}

func isDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func validIdentifier(value string) bool {
	for _, character := range value {
		if !(unicode.IsDigit(character) || unicode.IsLetter(character) || character == '-') || character > unicode.MaxASCII {
			return false
		}
	}
	return true
}

func compareParsed(left, right version) int {
	for _, pair := range [][2]uint64{{left.major, right.major}, {left.minor, right.minor}, {left.patch, right.patch}} {
		if pair[0] < pair[1] {
			return -1
		}
		if pair[0] > pair[1] {
			return 1
		}
	}
	if len(left.pre) == 0 && len(right.pre) == 0 {
		return 0
	}
	if len(left.pre) == 0 {
		return 1
	}
	if len(right.pre) == 0 {
		return -1
	}
	for index := 0; index < len(left.pre) && index < len(right.pre); index++ {
		if left.pre[index] == right.pre[index] {
			continue
		}
		leftIsNumber := isDigits(left.pre[index])
		rightIsNumber := isDigits(right.pre[index])
		if leftIsNumber && rightIsNumber {
			if len(left.pre[index]) < len(right.pre[index]) || (len(left.pre[index]) == len(right.pre[index]) && left.pre[index] < right.pre[index]) {
				return -1
			}
			return 1
		}
		if leftIsNumber {
			return -1
		}
		if rightIsNumber {
			return 1
		}
		if left.pre[index] < right.pre[index] {
			return -1
		}
		return 1
	}
	if len(left.pre) < len(right.pre) {
		return -1
	}
	if len(left.pre) > len(right.pre) {
		return 1
	}
	return 0
}

