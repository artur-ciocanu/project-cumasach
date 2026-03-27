package oci

import (
	"context"
	"fmt"
	"io"
	"slices"
	"sync"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/memory"
	"oras.land/oras-go/v2/registry/remote"
)

const (
	ConfigMediaType       = "application/vnd.agentskills.config.v1+json"
	ContentLayerMediaType = "application/vnd.agentskills.skill.content.v1.tar+gzip"
)

type Reference struct {
	Repository string
	Digest     string
}

func (r Reference) Canonical() string {
	return fmt.Sprintf("oci://%s@%s", r.Repository, r.Digest)
}

type PushOptions struct {
	Tag string
}

type FetchedArtifact struct {
	Reference  string
	Repository string
	Digest     string
	Config     []byte
	Archive    []byte
}

type Registry interface {
	PushTarget(context.Context, string) (oras.Target, error)
	ResolveReference(context.Context, string, string) (oras.ReadOnlyTarget, ocispec.Descriptor, error)
	ListTags(context.Context, string) ([]string, error)
}

type RemoteRegistry struct {
	PlainHTTP bool
}

func (r RemoteRegistry) PushTarget(_ context.Context, repository string) (oras.Target, error) {
	repo, err := remote.NewRepository(repository)
	if err != nil {
		return nil, fmt.Errorf("create remote repository %q: %w", repository, err)
	}
	repo.PlainHTTP = r.PlainHTTP
	return repo, nil
}

func (r RemoteRegistry) ResolveReference(ctx context.Context, repository, reference string) (oras.ReadOnlyTarget, ocispec.Descriptor, error) {
	repo, err := remote.NewRepository(repository)
	if err != nil {
		return nil, ocispec.Descriptor{}, fmt.Errorf("create remote repository %q: %w", repository, err)
	}
	repo.PlainHTTP = r.PlainHTTP

	desc, err := repo.Resolve(ctx, reference)
	if err != nil {
		return nil, ocispec.Descriptor{}, fmt.Errorf("resolve reference %q in %q: %w", reference, repository, err)
	}

	return repo, desc, nil
}

func (r RemoteRegistry) ListTags(ctx context.Context, repository string) ([]string, error) {
	repo, err := remote.NewRepository(repository)
	if err != nil {
		return nil, fmt.Errorf("create remote repository %q: %w", repository, err)
	}
	repo.PlainHTTP = r.PlainHTTP

	var tags []string
	if err := repo.Tags(ctx, "", func(batch []string) error {
		tags = append(tags, batch...)
		return nil
	}); err != nil {
		return nil, fmt.Errorf("list tags in %q: %w", repository, err)
	}

	slices.Sort(tags)
	return tags, nil
}

type StoredManifest struct {
	ManifestDescriptor ocispec.Descriptor
	ConfigDescriptor   ocispec.Descriptor
	LayerDescriptor    ocispec.Descriptor
	ConfigMediaType    string
	LayerMediaType     string
}

type MemoryRegistry struct {
	mu        sync.Mutex
	stores    map[string]*memory.Store
	manifests map[string]map[string]StoredManifest
	tags      map[string]map[string]struct{}
}

func NewMemoryRegistry() *MemoryRegistry {
	return &MemoryRegistry{
		stores:    make(map[string]*memory.Store),
		manifests: make(map[string]map[string]StoredManifest),
		tags:      make(map[string]map[string]struct{}),
	}
}

func (r *MemoryRegistry) PushTarget(_ context.Context, repository string) (oras.Target, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	return &memoryPushTarget{
		store:     r.storeLocked(repository),
		recordTag: func(tag string) { r.recordTag(repository, tag) },
	}, nil
}

func (r *MemoryRegistry) ResolveReference(ctx context.Context, repository, reference string) (oras.ReadOnlyTarget, ocispec.Descriptor, error) {
	r.mu.Lock()
	store := r.storeLocked(repository)
	if byRepo, ok := r.manifests[repository]; ok {
		if entry, ok := byRepo[reference]; ok {
			r.mu.Unlock()
			return store, entry.ManifestDescriptor, nil
		}
	}
	r.mu.Unlock()

	desc, err := store.Resolve(ctx, reference)
	if err != nil {
		return nil, ocispec.Descriptor{}, fmt.Errorf("resolve reference %q in %q: %w", reference, repository, err)
	}

	return store, desc, nil
}

func (r *MemoryRegistry) Resolve(repository, digest string) (StoredManifest, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	byRepo, ok := r.manifests[repository]
	if !ok {
		return StoredManifest{}, fmt.Errorf("repository %q not found", repository)
	}
	entry, ok := byRepo[digest]
	if !ok {
		return StoredManifest{}, fmt.Errorf("manifest %q not found in %q", digest, repository)
	}
	return entry, nil
}

func (r *MemoryRegistry) ListTags(_ context.Context, repository string) ([]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.stores[repository]; !ok {
		return nil, fmt.Errorf("repository %q not found", repository)
	}

	var tags []string
	for tag := range r.tags[repository] {
		tags = append(tags, tag)
	}
	slices.Sort(tags)
	return tags, nil
}

func (r *MemoryRegistry) recordManifest(repository string, entry StoredManifest) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.storeLocked(repository)
	if _, ok := r.manifests[repository]; !ok {
		r.manifests[repository] = make(map[string]StoredManifest)
	}
	r.manifests[repository][entry.ManifestDescriptor.Digest.String()] = entry
}

func (r *MemoryRegistry) recordTag(repository, tag string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.storeLocked(repository)
	if _, ok := r.tags[repository]; !ok {
		r.tags[repository] = make(map[string]struct{})
	}
	r.tags[repository][tag] = struct{}{}
}

type memoryPushTarget struct {
	store     *memory.Store
	recordTag func(string)
}

func (t *memoryPushTarget) Exists(ctx context.Context, desc ocispec.Descriptor) (bool, error) {
	return t.store.Exists(ctx, desc)
}

func (t *memoryPushTarget) Fetch(ctx context.Context, desc ocispec.Descriptor) (io.ReadCloser, error) {
	return t.store.Fetch(ctx, desc)
}

func (t *memoryPushTarget) Push(ctx context.Context, expected ocispec.Descriptor, reader io.Reader) error {
	return t.store.Push(ctx, expected, reader)
}

func (t *memoryPushTarget) Resolve(ctx context.Context, reference string) (ocispec.Descriptor, error) {
	return t.store.Resolve(ctx, reference)
}

func (t *memoryPushTarget) Tag(ctx context.Context, desc ocispec.Descriptor, reference string) error {
	if err := t.store.Tag(ctx, desc, reference); err != nil {
		return err
	}
	t.recordTag(reference)
	return nil
}

func (r *MemoryRegistry) storeLocked(repository string) *memory.Store {
	store, ok := r.stores[repository]
	if !ok {
		store = memory.New()
		r.stores[repository] = store
	}
	return store
}
