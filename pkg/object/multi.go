package object

import (
	"fmt"
	"io"
	"strings"
)

type MultiCloudStorage struct {
	Stores map[string]ObjectStorage
}

func (m *MultiCloudStorage) parseKey(key string) (string, string, error) {
	parts := strings.SplitN(key, ":", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid key format (expected provider:key): %s", key)
	}
	return parts[0], parts[1], nil
}

func (m *MultiCloudStorage) Get(key string, off, limit int64, getters ...AttrGetter) (io.ReadCloser, error) {
	provider, realKey, err := m.parseKey(key)
	if err != nil {
		return nil, err
	}
	backend, ok := m.Stores[provider]
	if !ok {
		return nil, fmt.Errorf("provider %s not found", provider)
	}
	return backend.Get(realKey, off, limit, getters...)
}

func (m *MultiCloudStorage) Put(key string, in io.Reader, getters ...AttrGetter) error {
	provider, realKey, err := m.parseKey(key)
	if err != nil {
		return err
	}
	backend, ok := m.Stores[provider]
	if !ok {
		return fmt.Errorf("provider %s not found", provider)
	}
	return backend.Put(realKey, in, getters...)
}

func (m *MultiCloudStorage) Delete(key string, getters ...AttrGetter) error {
	provider, realKey, err := m.parseKey(key)
	if err != nil {
		return err
	}
	backend, ok := m.Stores[provider]
	if !ok {
		return fmt.Errorf("provider %s not found", provider)
	}
	return backend.Delete(realKey, getters...)
}

func (m *MultiCloudStorage) Head(key string) (Object, error) {
	provider, realKey, err := m.parseKey(key)
	if err != nil {
		return nil, err
	}
	backend, ok := m.Stores[provider]
	if !ok {
		return nil, fmt.Errorf("provider %s not found", provider)
	}
	return backend.Head(realKey)
}

func (m *MultiCloudStorage) List(prefix, start, token, delimiter string, limit int64, followLink bool) ([]Object, bool, string, error) {
	// You can implement cross-provider listing later.
	return nil, false, "", fmt.Errorf("MultiCloudStorage does not support List")
}

func (m *MultiCloudStorage) Copy(dst, src string) error {
	dstProvider, dstKey, err := m.parseKey(dst)
	if err != nil {
		return err
	}
	srcProvider, srcKey, err := m.parseKey(src)
	if err != nil {
		return err
	}
	if dstProvider != srcProvider {
		return fmt.Errorf("Cross-provider Copy not supported: %s -> %s", srcProvider, dstProvider)
	}
	backend, ok := m.Stores[dstProvider]
	if !ok {
		return fmt.Errorf("provider %s not found", dstProvider)
	}
	return backend.Copy(dstKey, srcKey)
}

func (m *MultiCloudStorage) String() string {
	return "multi://"
}

func (m *MultiCloudStorage) Create() error {
	for _, backend := range m.Stores {
		if err := backend.Create(); err != nil {
			return err
		}
	}
	return nil
}

func (m *MultiCloudStorage) Limits() Limits {
	// Return conservative/default limits
	return Limits{
		IsSupportMultipartUpload: false,
	}
}