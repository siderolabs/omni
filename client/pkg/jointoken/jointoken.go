// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package jointoken implements siderolink jointoken parser.
package jointoken

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/jxskiss/base62"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// JoinTokenLen number of random bytes to be encoded in the join token.
// The real length of the token will depend on the base62 encoding,
// whose lengths happpens to be non-deterministic.
const JoinTokenLen = 32

// FingerprintLen is the number of base62 characters kept from the token digest.
const FingerprintLen = 16

// MaxEncodedTokenLen is the maximum length of the encoded join token.
//
// The token reaches the machine through the kernel command line, which is size limited,
// so the amount of data packed into it has to be bounded.
const MaxEncodedTokenLen = 1024

// The caps below mirror the ones the state layer applies to the resource labels, so that a label
// Omni would accept on a machine is not rejected here for a different reason.
// In practice the total token length is the binding constraint.
const (
	// MaxLabelKeyLength caps the byte length of a label key.
	MaxLabelKeyLength = 1024

	// MaxLabelValueLength caps the byte length of a label value.
	MaxLabelValueLength = 16 * 1024
)

// Generate the random join token string.
func Generate() (string, error) {
	b := make([]byte, JoinTokenLen)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to read random bytes: %w", err)
	}

	token := base62.EncodeToString(b)

	return token, nil
}

// Fingerprint derives the non-secret handle of a join token from its secret value.
//
// Omni publishes the same fingerprint as a label on the JoinTokenStatus resource, which lets a
// machine tell Omni which join token signed its data without putting the secret on the wire.
func Fingerprint(token string) string {
	sum := sha256.Sum256([]byte(token))

	encoded := base62.EncodeToString(sum[:])

	// base62 lengths are not deterministic: leading zero bytes shorten the result.
	if len(encoded) < FingerprintLen {
		return encoded
	}

	return encoded[:FingerprintLen]
}

// ExtraData is the type of the extra token data.
type ExtraData map[string]string

// allowedExtraDataKeys is the set of system keys the extra data may carry.
//
// Any other omni.sidero.dev/ prefixed key is rejected: user labels belong in the Labels field, so
// they can never be confused with the system data Omni acts on.
var allowedExtraDataKeys = map[string]struct{}{
	omni.LabelInfraProviderID: {},
	omni.LabelMachineRequest:  {},
}

const (
	v1Prefix = "v1:"
	v2Prefix = "v2:"
	v3Prefix = "v3:"
)

const (
	// VersionPlain is the random token string.
	VersionPlain = "plain"
	// Version1 is the signed token that contains extra data.
	Version1 = "1"
	// Version2 is the same as version 1, but the provider uses individual tokens.
	Version2 = "2"
	// Version3 is the same as version 2, and additionally carries the user labels and the
	// fingerprint of the join token which signed them.
	Version3 = "3"
)

// NewPlain token creates the token without extra data.
func NewPlain(token string) JoinToken {
	return JoinToken{
		token:   token,
		Version: VersionPlain,
	}
}

// NewWithExtraData creates the token with extra data.
func NewWithExtraData(token, version string, extraData map[string]string) (JoinToken, error) {
	if _, err := versionToPrefix(version); err != nil {
		return JoinToken{}, err
	}

	return newSigned(JoinToken{
		ExtraData: extraData,
		token:     token,
		Version:   version,
	})
}

// NewWithLabels creates a version 3 token which carries the user labels.
//
// The fingerprint of the join token is embedded so Omni can resolve which join token signed the
// data. It is omitted for the infra provider tokens: those are resolved by the provider ID, and
// the provider secret is not the token the machine is given.
func NewWithLabels(token string, labels, extraData map[string]string) (JoinToken, error) {
	t := JoinToken{
		ExtraData: extraData,
		Labels:    labels,
		token:     token,
		Version:   Version3,
	}

	if _, ok := extraData[omni.LabelInfraProviderID]; !ok {
		t.TokenFingerprint = Fingerprint(token)
	}

	return newSigned(t)
}

func newSigned(t JoinToken) (JoinToken, error) {
	if err := t.validate(); err != nil {
		return JoinToken{}, err
	}

	var err error

	if t.Signature, err = t.signature(t.token); err != nil {
		return t, err
	}

	// the encoded form is what ends up in the kernel command line, so that is the thing to bound
	encoded, err := t.Encode()
	if err != nil {
		return t, err
	}

	if len(encoded) > MaxEncodedTokenLen {
		return JoinToken{}, fmt.Errorf(
			"the encoded join token is too long: %d bytes (max %d), reduce the number or the size of the labels",
			len(encoded), MaxEncodedTokenLen,
		)
	}

	return t, nil
}

// JoinToken is the siderolink join token.
// Custom type adds methods for encoding/decoding extra data from the token.
type JoinToken struct {
	ExtraData ExtraData `json:"extra_data"`

	// Labels are the user defined machine labels, supported starting from Version3.
	Labels map[string]string `json:"labels,omitempty"`

	// TokenFingerprint identifies the join token which signed this data, supported starting from
	// Version3. See Fingerprint.
	TokenFingerprint string `json:"token_fp,omitempty"`

	token   string
	Version string `json:"-"`

	Signature []byte `json:"signature"`
}

// signedPayloadV3 is the message the version 3 signature is computed over.
//
// It is a distinct type from JoinToken so the version 1 and 2 payload - the bare extra data -
// stays byte identical, keeping already issued tokens valid.
type signedPayloadV3 struct {
	ExtraData        ExtraData         `json:"extra_data"`
	Labels           map[string]string `json:"labels,omitempty"`
	Version          string            `json:"version"`
	TokenFingerprint string            `json:"token_fp,omitempty"`
}

// signaturePayload returns the message to authenticate for the token version.
func (t JoinToken) signaturePayload() ([]byte, error) {
	switch t.Version {
	case Version3:
		return json.Marshal(signedPayloadV3{
			Version:          t.Version,
			ExtraData:        t.ExtraData,
			Labels:           t.Labels,
			TokenFingerprint: t.TokenFingerprint,
		})
	case Version1, Version2:
		return json.Marshal(t.ExtraData)
	case VersionPlain, "":
		return nil, fmt.Errorf("version %q is not signed", VersionPlain)
	default:
		return nil, fmt.Errorf("unsupported version %q", t.Version)
	}
}

func (t JoinToken) signature(token string) ([]byte, error) {
	data, err := t.signaturePayload()
	if err != nil {
		return nil, err
	}

	mac := hmac.New(sha256.New, []byte(token))

	if _, err = mac.Write(data); err != nil {
		return nil, err
	}

	return mac.Sum(nil), nil
}

// validate checks that the token only carries data Omni knows how to act on.
func (t JoinToken) validate() error {
	for key := range t.ExtraData {
		if !strings.HasPrefix(key, omni.SystemLabelPrefix) {
			continue
		}

		if _, ok := allowedExtraDataKeys[key]; !ok {
			return fmt.Errorf("extra data key %q is not supported", key)
		}
	}

	for key, value := range t.Labels {
		if strings.HasPrefix(key, omni.SystemLabelPrefix) {
			return fmt.Errorf("label %q is not allowed: the %q prefix is reserved for the system labels", key, omni.SystemLabelPrefix)
		}

		if err := validateLabel(key, value); err != nil {
			return err
		}
	}

	return nil
}

func validateLabel(key, value string) error {
	if key == "" {
		return errors.New("label key must not be empty")
	}

	if len(key) > MaxLabelKeyLength {
		return fmt.Errorf("label key is too long: %d bytes (max %d)", len(key), MaxLabelKeyLength)
	}

	if strings.ContainsFunc(key, unicode.IsControl) {
		return fmt.Errorf("label key %q must not contain control characters", key)
	}

	if len(value) > MaxLabelValueLength {
		return fmt.Errorf("label value for key %q is too long: %d bytes (max %d)", key, len(value), MaxLabelValueLength)
	}

	if strings.ContainsFunc(value, unicode.IsControl) {
		return fmt.Errorf("label value for key %q must not contain control characters", key)
	}

	return nil
}

// Parse reads string into token.
func Parse(value string) (JoinToken, error) {
	var res JoinToken

	res.Version = VersionPlain

	var prefix string

	switch {
	case strings.HasPrefix(value, v1Prefix):
		res.Version = Version1

		prefix = v1Prefix
	case strings.HasPrefix(value, v2Prefix):
		res.Version = Version2

		prefix = v2Prefix
	case strings.HasPrefix(value, v3Prefix):
		res.Version = Version3

		prefix = v3Prefix
	}

	if res.Version != VersionPlain {
		data, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(value, prefix))
		if err != nil {
			return res, err
		}

		return res, json.Unmarshal(data, &res)
	}

	res.token = value
	res.Version = VersionPlain

	return res, nil
}

// IsValid checks the signature or plain token.
func (t JoinToken) IsValid(token string) bool {
	if t.Signature == nil {
		return t.token == token
	}

	mac, err := t.signature(token)
	if err != nil {
		return false
	}

	return hmac.Equal(mac, t.Signature)
}

// Encode the token into string.
func (t JoinToken) Encode() (string, error) {
	if t.Version == VersionPlain || t.Version == "" {
		return t.token, nil
	}

	data, err := json.Marshal(t)
	if err != nil {
		return "", err
	}

	prefix, err := versionToPrefix(t.Version)
	if err != nil {
		return "", err
	}

	return prefix + base64.StdEncoding.EncodeToString(data), nil
}

func versionToPrefix(version string) (string, error) {
	switch version {
	case Version1:
		return v1Prefix, nil
	case Version2:
		return v2Prefix, nil
	case Version3:
		return v3Prefix, nil
	case VersionPlain:
		return "", fmt.Errorf("version %q does not have a prefix", VersionPlain)
	default:
		return "", fmt.Errorf("unsupported version %q", version)
	}
}
