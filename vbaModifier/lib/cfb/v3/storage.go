package v3

import (
	"fmt"
)

// Storage represents a storage (directory) within the CFB file
// Provides methods to navigate the directory tree and access streams
type Storage struct {
	reader *Reader
	entry  *DirectoryEntry
}

// OpenStream opens a stream by name within this storage
func (s *Storage) OpenStream(name string) (Stream, error) {
	entry := s.findChild(name)
	if entry == nil {
		return nil, fmt.Errorf("%w: stream %q not found in storage %q",
			ErrStreamNotFound, name, s.entry.Name)
	}

	if !entry.IsStream() {
		return nil, fmt.Errorf("%w: %q is a %s, not a stream",
			ErrNotAStream, name, entry.Type)
	}

	return newStream(s.reader, entry)
}

// OpenStorage opens a sub-storage by name within this storage
func (s *Storage) OpenStorage(name string) (*Storage, error) {
	entry := s.findChild(name)
	if entry == nil {
		return nil, fmt.Errorf("%w: storage %q not found in storage %q",
			ErrStorageNotFound, name, s.entry.Name)
	}

	if !entry.IsStorage() {
		return nil, fmt.Errorf("%w: %q is a %s, not a storage",
			ErrNotAStorage, name, entry.Type)
	}

	return &Storage{
		reader: s.reader,
		entry:  entry,
	}, nil
}

// ListStreams returns the names of all streams in this storage
func (s *Storage) ListStreams() []string {
	var streams []string
	s.iterateChildren(func(entry *DirectoryEntry) {
		if entry.IsStream() {
			streams = append(streams, entry.Name)
		}
	})
	return streams
}

// ListStorages returns the names of all sub-storages in this storage
func (s *Storage) ListStorages() []string {
	var storages []string
	s.iterateChildren(func(entry *DirectoryEntry) {
		if entry.IsStorage() && entry.Type != EntryTypeRoot {
			storages = append(storages, entry.Name)
		}
	})
	return storages
}

// List returns the names of all entries (streams and storages) in this storage
func (s *Storage) List() []string {
	var entries []string
	s.iterateChildren(func(entry *DirectoryEntry) {
		entries = append(entries, entry.Name)
	})
	return entries
}

// RemoveStream removes a stream by name from this storage
func (s *Storage) RemoveStream(name string) error {
	entry := s.findChild(name)
	if entry == nil {
		return fmt.Errorf("%w: %q not found in storage %q",
			ErrStreamNotFound, name, s.entry.Name)
	}

	if !entry.IsStream() {
		return fmt.Errorf("%w: %q is a %s, not a stream",
			ErrNotAStream, name, entry.Type)
	}

	// 1. Free the sector chain in FAT or MiniFAT
	if entry.Size > 0 {
		if entry.Size < uint64(s.reader.header.MiniStreamCutoff) {
			s.reader.freeMiniChain(entry.StartSector)
		} else {
			s.reader.freeChain(entry.StartSector)
		}
	}

	// 2. Remove from the red-black tree
	// We'll use a simplified approach: collect all children, remove the target,
	// and rebuild the tree for this storage.
	var children []*DirectoryEntry
	s.iterateChildren(func(child *DirectoryEntry) {
		if child.Name != name {
			children = append(children, child)
		}
	})

	// Mark the removed entry as invalid and clear its properties
	entry.Type = EntryTypeInvalid
	entry.Name = ""
	entry.NameLength = 0
	entry.LeftSibling = NOSTREAM
	entry.RightSibling = NOSTREAM
	entry.Child = NOSTREAM
	entry.StartSector = 0
	entry.Size = 0
	entry.LeftEntry = nil
	entry.RightEntry = nil
	entry.ChildEntry = nil

	// Rebuild the tree for this storage
	if len(children) == 0 {
		s.entry.Child = NOSTREAM
		s.entry.ChildEntry = nil
	} else {
		// Rebuild as a simple right-leaning tree (valid RB-tree if all black)
		for i := 0; i < len(children); i++ {
			child := children[i]
			child.Color = ColorBlack
			child.LeftSibling = NOSTREAM
			child.LeftEntry = nil

			if i < len(children)-1 {
				nextID := s.reader.findEntryID(children[i+1])
				child.RightSibling = nextID
				child.RightEntry = children[i+1]
			} else {
				child.RightSibling = NOSTREAM
				child.RightEntry = nil
			}
		}

		// Update parent's child pointer
		firstID := s.reader.findEntryID(children[0])
		s.entry.Child = firstID
		s.entry.ChildEntry = children[0]
	}

	return nil
}

// ReplaceStream replaces stream content in-place without reallocating sector chains.
// The replacement payload must fit in the currently allocated stream capacity.
func (s *Storage) ReplaceStream(name string, payload []byte) error {
	entry := s.findChild(name)
	if entry == nil {
		return fmt.Errorf("%w: %q not found in storage %q",
			ErrStreamNotFound, name, s.entry.Name)
	}
	if !entry.IsStream() {
		return fmt.Errorf("%w: %q is a %s, not a stream",
			ErrNotAStream, name, entry.Type)
	}
	return s.reader.replaceStream(entry, payload)
}

// Name returns the name of this storage
func (s *Storage) Name() string {
	return s.entry.Name
}

// findChild searches for a child entry by name
// Uses the red-black tree structure to search siblings
func (s *Storage) findChild(name string) *DirectoryEntry {
	if s.entry.ChildEntry == nil {
		return nil
	}
	return s.searchTree(s.entry.ChildEntry, name)
}

// searchTree recursively searches the red-black tree for an entry by name
func (s *Storage) searchTree(node *DirectoryEntry, name string) *DirectoryEntry {
	if node == nil {
		return nil
	}

	// Check current node
	if node.Name == name {
		return node
	}

	// Search left sibling subtree
	if left := s.searchTree(node.LeftEntry, name); left != nil {
		return left
	}

	// Search right sibling subtree
	if right := s.searchTree(node.RightEntry, name); right != nil {
		return right
	}

	return nil
}

// iterateChildren calls the given function for each child entry
func (s *Storage) iterateChildren(fn func(*DirectoryEntry)) {
	if s.entry.ChildEntry == nil {
		return
	}
	s.traverseTree(s.entry.ChildEntry, fn)
}

// traverseTree recursively traverses the red-black tree and calls fn for each node
func (s *Storage) traverseTree(node *DirectoryEntry, fn func(*DirectoryEntry)) {
	if node == nil {
		return
	}

	// Traverse left subtree
	s.traverseTree(node.LeftEntry, fn)

	// Visit current node
	fn(node)

	// Traverse right subtree
	s.traverseTree(node.RightEntry, fn)
}

// GetEntry returns the directory entry for this storage
func (s *Storage) GetEntry() *DirectoryEntry {
	return s.entry
}

// String returns a string representation of the storage
func (s *Storage) String() string {
	childCount := len(s.List())
	return fmt.Sprintf("Storage{Name: %q, Children: %d}", s.entry.Name, childCount)
}

// Exists checks if a child entry with the given name exists
func (s *Storage) Exists(name string) bool {
	return s.findChild(name) != nil
}

// StreamExists checks if a stream with the given name exists
func (s *Storage) StreamExists(name string) bool {
	entry := s.findChild(name)
	return entry != nil && entry.IsStream()
}

// StorageExists checks if a sub-storage with the given name exists
func (s *Storage) StorageExists(name string) bool {
	entry := s.findChild(name)
	return entry != nil && entry.IsStorage()
}

// GetAllEntries returns all child directory entries
func (s *Storage) GetAllEntries() []*DirectoryEntry {
	var entries []*DirectoryEntry
	s.iterateChildren(func(entry *DirectoryEntry) {
		entries = append(entries, entry)
	})
	return entries
}
