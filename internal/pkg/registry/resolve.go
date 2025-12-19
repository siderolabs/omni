package registry

import (
    "context"
    "fmt"

    "github.com/google/go-containerregistry/pkg/name"
    "github.com/google/go-containerregistry/pkg/v1/remote"
)

// ResolveDigest resolves an image tag to a content-addressable reference.
// Example:
//   "registry.k8s.io/kube-apiserver:v1.34.1" ->
//   "registry.k8s.io/kube-apiserver:v1.34.1@sha256:<digest>"
func ResolveDigest(ctx context.Context, imageWithTag string) (string, error) {
    if _, err := name.NewDigest(imageWithTag); err == nil {
        return imageWithTag, nil
    }

    // Parse the input as a tag (repo:tag). This gives a structured reference.
    ref, err := name.NewTag(imageWithTag)
    if err != nil {
        return "", fmt.Errorf("parsing image tag %q: %w", imageWithTag, err)
    }

    if desc, err := remote.Head(ref, remote.WithContext(ctx)); err == nil && desc != nil {
        return fmt.Sprintf("%s@%s", ref.String(), desc.Digest.String()), nil
    }

    img, err := remote.Image(ref, remote.WithContext(ctx))
    if err != nil {
        return "", fmt.Errorf("fetching image %s: %w", ref.String(), err)
    }
    digest, err := img.Digest()
    if err != nil {
        return "", fmt.Errorf("getting digest for %s: %w", ref.String(), err)
    }
    return fmt.Sprintf("%s@%s", ref.String(), digest.String()), nil
}