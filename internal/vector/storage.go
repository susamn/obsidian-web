package vector

import (
	"bufio"
	"bytes"
	"encoding/gob"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

const (
	vectorsFileName  = "vectors.bin"
	metadataFileName = "metadata.gob"
	indexFileName    = "index.gob"
)

// storage handles disk persistence for vectors and metadata.
type storage struct {
	mu          sync.RWMutex
	path        string
	dimension   int
	vectorsFile *os.File

	// In-memory index of vector positions in the file
	offsets  map[string]int64
	deleted  map[string]bool
	metadata map[string]map[string]string
}

// openStorage opens or creates a storage directory.
func openStorage(path string, dimension int) (*storage, error) {
	s := &storage{
		path:      path,
		dimension: dimension,
		offsets:   make(map[string]int64),
		deleted:   make(map[string]bool),
		metadata:  make(map[string]map[string]string),
	}

	// Open or create vectors file
	vectorsPath := filepath.Join(path, vectorsFileName)
	f, err := os.OpenFile(vectorsPath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open vectors file: %w", err)
	}
	s.vectorsFile = f

	// Load index if exists
	if err := s.loadIndex(); err != nil {
		f.Close()
		return nil, fmt.Errorf("failed to load index: %w", err)
	}

	// Load metadata if exists
	if err := s.loadMetadata(); err != nil {
		f.Close()
		return nil, fmt.Errorf("failed to load metadata: %w", err)
	}

	return s, nil
}

// loadIndex loads the offset index from disk.
func (s *storage) loadIndex() error {
	indexPath := filepath.Join(s.path, indexFileName)
	f, err := os.Open(indexPath)
	if os.IsNotExist(err) {
		return nil // No existing index
	}
	if err != nil {
		return err
	}
	defer f.Close()

	type indexData struct {
		Offsets map[string]int64
		Deleted map[string]bool
	}

	var data indexData
	if err := gob.NewDecoder(f).Decode(&data); err != nil {
		return err
	}

	s.offsets = data.Offsets
	s.deleted = data.Deleted
	if s.deleted == nil {
		s.deleted = make(map[string]bool)
	}
	return nil
}

// saveIndex persists the offset index to disk.
func (s *storage) saveIndex() error {
	indexPath := filepath.Join(s.path, indexFileName)
	f, err := os.Create(indexPath)
	if err != nil {
		return err
	}
	defer f.Close()

	type indexData struct {
		Offsets map[string]int64
		Deleted map[string]bool
	}

	data := indexData{
		Offsets: s.offsets,
		Deleted: s.deleted,
	}
	return gob.NewEncoder(f).Encode(&data)
}

// loadMetadata loads the metadata map from disk.
func (s *storage) loadMetadata() error {
	metadataPath := filepath.Join(s.path, metadataFileName)
	f, err := os.Open(metadataPath)
	if os.IsNotExist(err) {
		return nil // No existing metadata
	}
	if err != nil {
		return err
	}
	defer f.Close()

	return gob.NewDecoder(f).Decode(&s.metadata)
}

// saveMetadata persists the metadata map to disk.
func (s *storage) saveMetadata() error {
	metadataPath := filepath.Join(s.path, metadataFileName)
	f, err := os.Create(metadataPath)
	if err != nil {
		return err
	}
	defer f.Close()

	return gob.NewEncoder(f).Encode(s.metadata)
}

// save persists a single vector to storage.
func (s *storage) save(v *Vector) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Write vector to end of file
	offset, err := s.vectorsFile.Seek(0, io.SeekEnd)
	if err != nil {
		return err
	}

	// Write ID
	if err := writeString(s.vectorsFile, v.ID); err != nil {
		return err
	}

	// Write dimension
	if err := writeInt32(s.vectorsFile, int32(len(v.Data))); err != nil {
		return err
	}

	// Write vector data
	for _, val := range v.Data {
		if err := writeFloat32(s.vectorsFile, val); err != nil {
			return err
		}
	}

	// Mark old entry as deleted if exists
	if _, exists := s.offsets[v.ID]; exists {
		s.deleted[v.ID] = true
	}

	// Update index
	s.offsets[v.ID] = offset
	delete(s.deleted, v.ID)

	// Update metadata
	if v.Metadata != nil {
		s.metadata[v.ID] = v.Metadata
	} else {
		delete(s.metadata, v.ID)
	}

	// Persist index and metadata
	if err := s.saveIndex(); err != nil {
		return err
	}
	return s.saveMetadata()
}

// saveBatch persists multiple vectors efficiently.
func (s *storage) saveBatch(vectors []*Vector) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Create a buffered writer
	offset, err := s.vectorsFile.Seek(0, io.SeekEnd)
	if err != nil {
		return err
	}

	buf := bufio.NewWriter(s.vectorsFile)
	currentOffset := offset

	for _, v := range vectors {
		// Mark old entry as deleted if exists
		if _, exists := s.offsets[v.ID]; exists {
			s.deleted[v.ID] = true
		}

		startOffset := currentOffset

		// Write ID
		idBytes := []byte(v.ID)
		var intBuf bytes.Buffer
		writeInt32(&intBuf, int32(len(idBytes)))
		n, _ := buf.Write(intBuf.Bytes())
		currentOffset += int64(n)
		n, _ = buf.Write(idBytes)
		currentOffset += int64(n)

		// Write dimension
		intBuf.Reset()
		writeInt32(&intBuf, int32(len(v.Data)))
		n, _ = buf.Write(intBuf.Bytes())
		currentOffset += int64(n)

		// Write vector data
		var floatBuf bytes.Buffer
		for _, val := range v.Data {
			floatBuf.Reset()
			writeFloat32(&floatBuf, val)
			n, _ = buf.Write(floatBuf.Bytes())
			currentOffset += int64(n)
		}

		// Update index
		s.offsets[v.ID] = startOffset
		delete(s.deleted, v.ID)

		// Update metadata
		if v.Metadata != nil {
			s.metadata[v.ID] = v.Metadata
		} else {
			delete(s.metadata, v.ID)
		}
	}

	if err := buf.Flush(); err != nil {
		return err
	}

	// Persist index and metadata
	if err := s.saveIndex(); err != nil {
		return err
	}
	return s.saveMetadata()
}

// get retrieves a vector by ID.
func (s *storage) get(id string) (*Vector, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	offset, exists := s.offsets[id]
	if !exists || s.deleted[id] {
		return nil, ErrNotFound
	}

	if _, err := s.vectorsFile.Seek(offset, io.SeekStart); err != nil {
		return nil, err
	}

	// Read ID
	storedID, err := readString(s.vectorsFile)
	if err != nil {
		return nil, err
	}

	// Read dimension
	dim, err := readInt32(s.vectorsFile)
	if err != nil {
		return nil, err
	}

	// Read vector data
	data := make([]float32, dim)
	for i := range data {
		val, err := readFloat32(s.vectorsFile)
		if err != nil {
			return nil, err
		}
		data[i] = val
	}

	return &Vector{
		ID:       storedID,
		Data:     data,
		Metadata: s.metadata[id],
	}, nil
}

// delete marks a vector as deleted.
func (s *storage) delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.offsets[id]; !exists {
		return ErrNotFound
	}

	s.deleted[id] = true
	delete(s.metadata, id)

	// Persist changes
	if err := s.saveIndex(); err != nil {
		return err
	}
	return s.saveMetadata()
}

// loadAll loads all non-deleted vectors from storage.
func (s *storage) loadAll() ([]*Vector, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	vectors := make([]*Vector, 0, len(s.offsets)-len(s.deleted))

	for id, offset := range s.offsets {
		if s.deleted[id] {
			continue
		}

		if _, err := s.vectorsFile.Seek(offset, io.SeekStart); err != nil {
			return nil, err
		}

		// Read ID
		storedID, err := readString(s.vectorsFile)
		if err != nil {
			return nil, err
		}

		// Read dimension
		dim, err := readInt32(s.vectorsFile)
		if err != nil {
			return nil, err
		}

		// Read vector data
		data := make([]float32, dim)
		for i := range data {
			val, err := readFloat32(s.vectorsFile)
			if err != nil {
				return nil, err
			}
			data[i] = val
		}

		vectors = append(vectors, &Vector{
			ID:       storedID,
			Data:     data,
			Metadata: s.metadata[id],
		})
	}

	return vectors, nil
}

// compact rewrites the storage file to reclaim space from deleted entries.
func (s *storage) compact() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Nothing to compact
	if len(s.deleted) == 0 {
		return nil
	}

	// Load all active vectors
	vectors := make([]*Vector, 0, len(s.offsets)-len(s.deleted))
	for id, offset := range s.offsets {
		if s.deleted[id] {
			continue
		}

		if _, err := s.vectorsFile.Seek(offset, io.SeekStart); err != nil {
			return err
		}

		storedID, err := readString(s.vectorsFile)
		if err != nil {
			return err
		}

		dim, err := readInt32(s.vectorsFile)
		if err != nil {
			return err
		}

		data := make([]float32, dim)
		for i := range data {
			val, err := readFloat32(s.vectorsFile)
			if err != nil {
				return err
			}
			data[i] = val
		}

		vectors = append(vectors, &Vector{
			ID:       storedID,
			Data:     data,
			Metadata: s.metadata[id],
		})
	}

	// Close current file
	s.vectorsFile.Close()

	// Create new file
	vectorsPath := filepath.Join(s.path, vectorsFileName)
	tempPath := vectorsPath + ".tmp"
	f, err := os.Create(tempPath)
	if err != nil {
		// Reopen old file
		s.vectorsFile, _ = os.OpenFile(vectorsPath, os.O_RDWR, 0644)
		return err
	}

	// Write all vectors to new file
	newOffsets := make(map[string]int64)
	buf := bufio.NewWriter(f)

	for _, v := range vectors {
		offset, _ := f.Seek(0, io.SeekCurrent)
		// Account for buffered data
		offset += int64(buf.Buffered())
		newOffsets[v.ID] = offset

		// Write ID
		idBytes := []byte(v.ID)
		var intBuf bytes.Buffer
		writeInt32(&intBuf, int32(len(idBytes)))
		buf.Write(intBuf.Bytes())
		buf.Write(idBytes)

		// Write dimension
		intBuf.Reset()
		writeInt32(&intBuf, int32(len(v.Data)))
		buf.Write(intBuf.Bytes())

		// Write vector data
		var floatBuf bytes.Buffer
		for _, val := range v.Data {
			floatBuf.Reset()
			writeFloat32(&floatBuf, val)
			buf.Write(floatBuf.Bytes())
		}
	}

	if err := buf.Flush(); err != nil {
		f.Close()
		os.Remove(tempPath)
		s.vectorsFile, _ = os.OpenFile(vectorsPath, os.O_RDWR, 0644)
		return err
	}
	f.Close()

	// Replace old file with new
	if err := os.Rename(tempPath, vectorsPath); err != nil {
		os.Remove(tempPath)
		s.vectorsFile, _ = os.OpenFile(vectorsPath, os.O_RDWR, 0644)
		return err
	}

	// Reopen file
	s.vectorsFile, err = os.OpenFile(vectorsPath, os.O_RDWR, 0644)
	if err != nil {
		return err
	}

	// Update index
	s.offsets = newOffsets
	s.deleted = make(map[string]bool)

	// Persist updated index
	return s.saveIndex()
}

// close closes the storage.
func (s *storage) close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.vectorsFile != nil {
		return s.vectorsFile.Close()
	}
	return nil
}
