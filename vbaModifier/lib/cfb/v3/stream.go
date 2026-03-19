package v3

import (
	"fmt"
	"io"
)

// Stream represents a readable stream within a CFB file
// Implements io.Reader, io.Seeker, and io.Closer
type Stream interface {
	io.Reader
	io.Seeker
	io.Closer
	Size() int64
	Name() string
}

// oleStream is the internal implementation of Stream
type oleStream struct {
	reader   *Reader
	entry    *DirectoryEntry
	position int64
	data     []byte // Cached stream data
	closed   bool
}

// newStream creates a new stream for the given directory entry
func newStream(reader *Reader, entry *DirectoryEntry) (Stream, error) {
	if entry == nil {
		return nil, fmt.Errorf("nil directory entry")
	}

	if !entry.IsStream() {
		return nil, fmt.Errorf("%w: entry %q is type %s", ErrNotAStream, entry.Name, entry.Type)
	}

	stream := &oleStream{
		reader:   reader,
		entry:    entry,
		position: 0,
		closed:   false,
	}

	// Load the stream data
	if err := stream.loadData(); err != nil {
		return nil, fmt.Errorf("failed to load stream data: %w", err)
	}

	return stream, nil
}

// loadData loads the stream's data into memory
func (s *oleStream) loadData() error {
	// Empty stream
	if s.entry.Size == 0 {
		s.data = []byte{}
		return nil
	}

	// Determine if this is a mini stream or regular stream
	if s.entry.Size < uint64(s.reader.header.MiniStreamCutoff) {
		// Use mini stream
		data, err := s.reader.readMiniChain(s.entry.StartSector, s.entry.Size)
		if err != nil {
			return fmt.Errorf("failed to read mini stream: %w", err)
		}
		s.data = data
	} else {
		// Use regular stream
		data, err := s.reader.readChain(s.entry.StartSector, s.entry.Size)
		if err != nil {
			return fmt.Errorf("failed to read regular stream: %w", err)
		}
		s.data = data
	}

	return nil
}

// Read reads up to len(p) bytes into p
// Implements io.Reader interface
func (s *oleStream) Read(p []byte) (n int, err error) {
	if s.closed {
		return 0, ErrStreamClosed
	}

	// Check if we're at the end
	if s.position >= int64(len(s.data)) {
		return 0, io.EOF
	}

	// Calculate how much to read
	remaining := int64(len(s.data)) - s.position
	toRead := int64(len(p))
	if toRead > remaining {
		toRead = remaining
	}

	// Copy data
	n = copy(p, s.data[s.position:s.position+toRead])
	s.position += int64(n)

	// Return EOF if we've read everything
	if s.position >= int64(len(s.data)) {
		err = io.EOF
	}

	return n, err
}

// Seek sets the offset for the next Read
// Implements io.Seeker interface
func (s *oleStream) Seek(offset int64, whence int) (int64, error) {
	if s.closed {
		return 0, ErrStreamClosed
	}

	var newPos int64

	switch whence {
	case io.SeekStart:
		newPos = offset
	case io.SeekCurrent:
		newPos = s.position + offset
	case io.SeekEnd:
		newPos = int64(len(s.data)) + offset
	default:
		return 0, fmt.Errorf("invalid whence: %d", whence)
	}

	// Validate position
	if newPos < 0 {
		return 0, ErrInvalidSeekOffset
	}

	// Allow seeking beyond EOF (as per io.Seeker contract)
	s.position = newPos
	return s.position, nil
}

// Close closes the stream
// Implements io.Closer interface
func (s *oleStream) Close() error {
	if s.closed {
		return nil
	}
	s.closed = true
	s.data = nil // Release memory
	return nil
}

// Size returns the size of the stream in bytes
func (s *oleStream) Size() int64 {
	return int64(s.entry.Size)
}

// Name returns the name of the stream
func (s *oleStream) Name() string {
	return s.entry.Name
}

// ReadAll is a convenience method to read the entire stream
func (s *oleStream) ReadAll() ([]byte, error) {
	if s.closed {
		return nil, ErrStreamClosed
	}

	// Reset position to start
	s.position = 0

	// Return a copy of the data
	data := make([]byte, len(s.data))
	copy(data, s.data)
	return data, nil
}

// String returns a string representation of the stream
func (s *oleStream) String() string {
	return fmt.Sprintf("Stream{Name: %q, Size: %d, Position: %d, Closed: %v}",
		s.entry.Name, s.entry.Size, s.position, s.closed)
}
