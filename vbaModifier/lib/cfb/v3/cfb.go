package v3

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"unicode/utf16"
)

const (
	headerSize       = 512
	dirEntrySize     = 128
	miniStreamCutoff = 4096

	objTypeUnknown = 0
	objTypeStorage = 1
	objTypeStream  = 2
	objTypeRoot    = 5

	noStream       = 0xFFFFFFFF
	freeSect       = 0xFFFFFFFF
	endOfChain     = 0xFFFFFFFE
	fatSect        = 0xFFFFFFFD
	difatSect      = 0xFFFFFFFC
	maxRegularName = 31
)

var (
	errInvalidSignature = errors.New("cfb: invalid signature")
	errUnsupportedCFB   = errors.New("cfb: unsupported compound file variant")
	errEntryNotFound    = errors.New("cfb: entry not found")
)

type Reader struct {
	data           []byte
	sectorSize     int
	miniSectorSize int
	miniCutoff     uint32
	firstDirSector uint32
	firstMiniFAT   uint32
	miniFATCount   uint32
	firstDIFAT     uint32
	difatCount     uint32
	fatSectors     []uint32
	fat            []uint32
	miniFAT        []uint32
	root           *node
	rootMiniStream []byte
}

type Storage struct {
	reader *Reader
	node   *node
}

type streamReadCloser struct {
	*bytes.Reader
}

type node struct {
	name       string
	objType    uint8
	clsid      [16]byte
	stateBits  uint32
	createdAt  uint64
	modifiedAt uint64
	data       []byte
	parent     *node
	children   []*node
}

type header struct {
	SectorShift          uint16
	MiniSectorShift      uint16
	NumDirSectors        uint32
	NumFATSectors        uint32
	FirstDirSectorLoc    uint32
	MiniStreamCutoffSize uint32
	FirstMiniFATSector   uint32
	NumMiniFATSectors    uint32
	FirstDIFATSector     uint32
	NumDIFATSectors      uint32
	DIFAT                [109]uint32
}

type dirEntry struct {
	name       string
	objType    uint8
	left       uint32
	right      uint32
	child      uint32
	clsid      [16]byte
	stateBits  uint32
	createdAt  uint64
	modifiedAt uint64
	start      uint32
	size       uint64
}

func OpenBytes(data []byte) (*Reader, error) {
	if len(data) < headerSize {
		return nil, io.ErrUnexpectedEOF
	}
	hdr, err := parseHeader(data[:headerSize])
	if err != nil {
		return nil, err
	}
	reader := &Reader{
		data:           append([]byte(nil), data...),
		sectorSize:     1 << hdr.SectorShift,
		miniSectorSize: 1 << hdr.MiniSectorShift,
		miniCutoff:     hdr.MiniStreamCutoffSize,
		firstDirSector: hdr.FirstDirSectorLoc,
		firstMiniFAT:   hdr.FirstMiniFATSector,
		miniFATCount:   hdr.NumMiniFATSectors,
		firstDIFAT:     hdr.FirstDIFATSector,
		difatCount:     hdr.NumDIFATSectors,
	}
	reader.fatSectors, err = reader.collectFATSectors(hdr)
	if err != nil {
		return nil, err
	}
	reader.fat, err = reader.readFAT()
	if err != nil {
		return nil, err
	}
	dirs, err := reader.readDirectoryEntries()
	if err != nil {
		return nil, err
	}
	if len(dirs) == 0 {
		return nil, errors.New("cfb: empty directory stream")
	}
	reader.root, err = reader.buildTree(dirs)
	if err != nil {
		return nil, err
	}
	if reader.root.objType != objTypeRoot {
		return nil, errors.New("cfb: root entry missing")
	}
	if reader.miniFATCount > 0 && reader.firstMiniFAT != endOfChain {
		reader.miniFAT, err = reader.readMiniFAT()
		if err != nil {
			return nil, err
		}
	}
	reader.rootMiniStream, err = reader.readStreamData(reader.root, dirs[0].start, dirs[0].size)
	if err != nil {
		return nil, err
	}
	if err := reader.populateStreamData(reader.root, dirs); err != nil {
		return nil, err
	}
	return reader, nil
}

func (r *Reader) Close() error {
	r.data = nil
	r.fat = nil
	r.fatSectors = nil
	r.miniFAT = nil
	r.rootMiniStream = nil
	r.root = nil
	return nil
}

func (r *Reader) OpenRootStorage() (*Storage, error) {
	if r.root == nil {
		return nil, errors.New("cfb: closed reader")
	}
	return &Storage{reader: r, node: r.root}, nil
}

func (r *Reader) WriteTo(w io.Writer) (int64, error) {
	serialized, err := r.serialize()
	if err != nil {
		return 0, err
	}
	n, err := w.Write(serialized)
	return int64(n), err
}

func (s *Storage) OpenStorage(name string) (*Storage, error) {
	child := s.findChild(name, objTypeStorage, objTypeRoot)
	if child == nil {
		return nil, errEntryNotFound
	}
	return &Storage{reader: s.reader, node: child}, nil
}

func (s *Storage) OpenStream(name string) (io.ReadCloser, error) {
	child := s.findChild(name, objTypeStream)
	if child == nil {
		return nil, errEntryNotFound
	}
	return streamReadCloser{Reader: bytes.NewReader(child.data)}, nil
}

func (streamReadCloser) Close() error {
	return nil
}

func (s *Storage) ReplaceStream(name string, data []byte) error {
	child := s.findChild(name, objTypeStream)
	if child == nil {
		return errEntryNotFound
	}
	child.data = append([]byte(nil), data...)
	return nil
}

func (s *Storage) RemoveStream(name string) error {
	for idx, child := range s.node.children {
		if child.objType == objTypeStream && strings.EqualFold(child.name, name) {
			s.node.children = append(s.node.children[:idx], s.node.children[idx+1:]...)
			child.parent = nil
			return nil
		}
	}
	return errEntryNotFound
}

func (s *Storage) StreamExists(name string) bool {
	return s.findChild(name, objTypeStream) != nil
}

func (s *Storage) StorageExists(name string) bool {
	return s.findChild(name, objTypeStorage, objTypeRoot) != nil
}

func (s *Storage) ListStreams() []string {
	streams := make([]string, 0, len(s.node.children))
	for _, child := range s.node.children {
		if child.objType == objTypeStream {
			streams = append(streams, child.name)
		}
	}
	sort.Slice(streams, func(i, j int) bool {
		return strings.ToLower(streams[i]) < strings.ToLower(streams[j])
	})
	return streams
}

func (s *Storage) ListStorages() []string {
	storages := make([]string, 0, len(s.node.children))
	for _, child := range s.node.children {
		if child.objType == objTypeStorage || child.objType == objTypeRoot {
			storages = append(storages, child.name)
		}
	}
	sort.Slice(storages, func(i, j int) bool {
		return strings.ToLower(storages[i]) < strings.ToLower(storages[j])
	})
	return storages
}

func (s *Storage) findChild(name string, allowedTypes ...uint8) *node {
	for _, child := range s.node.children {
		if !strings.EqualFold(child.name, name) {
			continue
		}
		for _, allowed := range allowedTypes {
			if child.objType == allowed {
				return child
			}
		}
	}
	return nil
}

func parseHeader(buf []byte) (header, error) {
	var hdr header
	if !bytes.Equal(buf[:8], []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}) {
		return hdr, errInvalidSignature
	}
	if binary.LittleEndian.Uint16(buf[28:30]) != 0xFFFE {
		return hdr, errUnsupportedCFB
	}
	hdr.SectorShift = binary.LittleEndian.Uint16(buf[30:32])
	hdr.MiniSectorShift = binary.LittleEndian.Uint16(buf[32:34])
	if hdr.SectorShift != 9 || hdr.MiniSectorShift != 6 {
		return hdr, errUnsupportedCFB
	}
	hdr.NumDirSectors = binary.LittleEndian.Uint32(buf[40:44])
	hdr.NumFATSectors = binary.LittleEndian.Uint32(buf[44:48])
	hdr.FirstDirSectorLoc = binary.LittleEndian.Uint32(buf[48:52])
	hdr.MiniStreamCutoffSize = binary.LittleEndian.Uint32(buf[56:60])
	hdr.FirstMiniFATSector = binary.LittleEndian.Uint32(buf[60:64])
	hdr.NumMiniFATSectors = binary.LittleEndian.Uint32(buf[64:68])
	hdr.FirstDIFATSector = binary.LittleEndian.Uint32(buf[68:72])
	hdr.NumDIFATSectors = binary.LittleEndian.Uint32(buf[72:76])
	for i := 0; i < len(hdr.DIFAT); i++ {
		hdr.DIFAT[i] = binary.LittleEndian.Uint32(buf[76+i*4 : 80+i*4])
	}
	if hdr.MiniStreamCutoffSize == 0 {
		hdr.MiniStreamCutoffSize = miniStreamCutoff
	}
	return hdr, nil
}

func (r *Reader) collectFATSectors(hdr header) ([]uint32, error) {
	sectors := make([]uint32, 0, hdr.NumFATSectors)
	for _, sector := range hdr.DIFAT {
		if sector != freeSect {
			sectors = append(sectors, sector)
		}
	}
	current := hdr.FirstDIFATSector
	for i := uint32(0); i < hdr.NumDIFATSectors; i++ {
		if current == endOfChain || current == freeSect {
			break
		}
		sector, err := r.readSector(current)
		if err != nil {
			return nil, err
		}
		for offset := 0; offset < r.sectorSize-4; offset += 4 {
			value := binary.LittleEndian.Uint32(sector[offset : offset+4])
			if value != freeSect {
				sectors = append(sectors, value)
			}
		}
		current = binary.LittleEndian.Uint32(sector[r.sectorSize-4:])
	}
	if uint32(len(sectors)) < hdr.NumFATSectors {
		return nil, errors.New("cfb: incomplete DIFAT")
	}
	return sectors[:hdr.NumFATSectors], nil
}

func (r *Reader) readFAT() ([]uint32, error) {
	entriesPerSector := r.sectorSize / 4
	fat := make([]uint32, 0, len(r.fatSectors)*entriesPerSector)
	for _, sectorNum := range r.fatSectors {
		sector, err := r.readSector(sectorNum)
		if err != nil {
			return nil, err
		}
		for offset := 0; offset < len(sector); offset += 4 {
			fat = append(fat, binary.LittleEndian.Uint32(sector[offset:offset+4]))
		}
	}
	return fat, nil
}

func (r *Reader) readDirectoryEntries() ([]dirEntry, error) {
	buf, err := r.readChain(r.firstDirSector, 0, r.fat, r.sectorSize)
	if err != nil {
		return nil, err
	}
	count := len(buf) / dirEntrySize
	entries := make([]dirEntry, 0, count)
	for idx := 0; idx < count; idx++ {
		entryBuf := buf[idx*dirEntrySize : (idx+1)*dirEntrySize]
		nameLen := int(binary.LittleEndian.Uint16(entryBuf[64:66]))
		if nameLen < 2 || nameLen > 64 {
			nameLen = 0
		}
		entry := dirEntry{
			name:       decodeUTF16Name(entryBuf[:nameLen]),
			objType:    entryBuf[66],
			left:       binary.LittleEndian.Uint32(entryBuf[68:72]),
			right:      binary.LittleEndian.Uint32(entryBuf[72:76]),
			child:      binary.LittleEndian.Uint32(entryBuf[76:80]),
			stateBits:  binary.LittleEndian.Uint32(entryBuf[96:100]),
			createdAt:  binary.LittleEndian.Uint64(entryBuf[100:108]),
			modifiedAt: binary.LittleEndian.Uint64(entryBuf[108:116]),
			start:      binary.LittleEndian.Uint32(entryBuf[116:120]),
			size:       binary.LittleEndian.Uint64(entryBuf[120:128]),
		}
		copy(entry.clsid[:], entryBuf[80:96])
		entries = append(entries, entry)
	}
	return entries, nil
}

func (r *Reader) buildTree(entries []dirEntry) (*node, error) {
	nodes := make([]*node, len(entries))
	for idx, entry := range entries {
		nodes[idx] = &node{
			name:       entry.name,
			objType:    entry.objType,
			clsid:      entry.clsid,
			stateBits:  entry.stateBits,
			createdAt:  entry.createdAt,
			modifiedAt: entry.modifiedAt,
		}
	}
	var attachChildren func(parent *node, childIndex uint32) error
	attachChildren = func(parent *node, childIndex uint32) error {
		if childIndex == noStream {
			return nil
		}
		visited := make(map[uint32]struct{})
		var walk func(index uint32) error
		walk = func(index uint32) error {
			if index == noStream {
				return nil
			}
			if index >= uint32(len(entries)) {
				return fmt.Errorf("cfb: invalid directory index %d", index)
			}
			if _, seen := visited[index]; seen {
				return nil
			}
			visited[index] = struct{}{}
			entry := entries[index]
			if err := walk(entry.left); err != nil {
				return err
			}
			child := nodes[index]
			child.parent = parent
			parent.children = append(parent.children, child)
			if child.objType == objTypeStorage || child.objType == objTypeRoot {
				if err := attachChildren(child, entry.child); err != nil {
					return err
				}
			}
			return walk(entry.right)
		}
		return walk(childIndex)
	}
	root := nodes[0]
	if err := attachChildren(root, entries[0].child); err != nil {
		return nil, err
	}
	return root, nil
}

func (r *Reader) readMiniFAT() ([]uint32, error) {
	buf, err := r.readChain(r.firstMiniFAT, int(r.miniFATCount)*r.sectorSize, r.fat, r.sectorSize)
	if err != nil {
		return nil, err
	}
	miniFAT := make([]uint32, 0, len(buf)/4)
	for offset := 0; offset+4 <= len(buf); offset += 4 {
		miniFAT = append(miniFAT, binary.LittleEndian.Uint32(buf[offset:offset+4]))
	}
	return miniFAT, nil
}

func (r *Reader) populateStreamData(current *node, entries []dirEntry) error {
	entry := findEntry(entries, current.name, current.objType)
	if entry != nil && current.objType == objTypeStream {
		data, err := r.readStreamData(current, entry.start, entry.size)
		if err != nil {
			return err
		}
		current.data = data
	}
	for _, child := range current.children {
		if err := r.populateStreamData(child, entries); err != nil {
			return err
		}
	}
	return nil
}

func findEntry(entries []dirEntry, name string, objType uint8) *dirEntry {
	for idx := range entries {
		if entries[idx].objType == objType && entries[idx].name == name {
			return &entries[idx]
		}
	}
	return nil
}

func (r *Reader) readStreamData(entry *node, start uint32, size uint64) ([]byte, error) {
	if size == 0 || start == endOfChain || start == freeSect {
		return nil, nil
	}
	if entry.objType != objTypeRoot && size < uint64(r.miniCutoff) {
		return r.readChainFromMini(start, int(size))
	}
	return r.readChain(start, int(size), r.fat, r.sectorSize)
}

func (r *Reader) readChainFromMini(start uint32, size int) ([]byte, error) {
	if len(r.miniFAT) == 0 {
		return nil, errors.New("cfb: missing mini FAT")
	}
	buf := &bytes.Buffer{}
	current := start
	for current != endOfChain {
		startOffset := int(current) * r.miniSectorSize
		endOffset := startOffset + r.miniSectorSize
		if startOffset < 0 || endOffset > len(r.rootMiniStream) {
			return nil, io.ErrUnexpectedEOF
		}
		buf.Write(r.rootMiniStream[startOffset:endOffset])
		if int(current) >= len(r.miniFAT) {
			return nil, errors.New("cfb: invalid mini sector chain")
		}
		current = r.miniFAT[current]
	}
	result := buf.Bytes()
	if size > 0 && size < len(result) {
		result = result[:size]
	}
	return append([]byte(nil), result...), nil
}

func (r *Reader) readChain(start uint32, size int, table []uint32, unitSize int) ([]byte, error) {
	if start == endOfChain || start == freeSect {
		return nil, nil
	}
	buf := &bytes.Buffer{}
	current := start
	visited := make(map[uint32]struct{})
	for current != endOfChain {
		if _, seen := visited[current]; seen {
			return nil, errors.New("cfb: sector cycle detected")
		}
		visited[current] = struct{}{}
		sector, err := r.readSector(current)
		if err != nil {
			return nil, err
		}
		buf.Write(sector)
		if int(current) >= len(table) {
			return nil, errors.New("cfb: invalid FAT chain")
		}
		current = table[current]
	}
	data := buf.Bytes()
	if size > 0 && size < len(data) {
		data = data[:size]
	}
	if size == 0 && len(data)%unitSize != 0 {
		return nil, io.ErrUnexpectedEOF
	}
	return append([]byte(nil), data...), nil
}

func (r *Reader) readSector(index uint32) ([]byte, error) {
	offset := headerSize + int(index)*r.sectorSize
	if offset < headerSize || offset+r.sectorSize > len(r.data) {
		return nil, io.ErrUnexpectedEOF
	}
	return r.data[offset : offset+r.sectorSize], nil
}

type serialEntry struct {
	node       *node
	index      uint32
	left       uint32
	right      uint32
	child      uint32
	start      uint32
	size       uint64
	miniStart  uint32
	regularLen int
}

type sectorChain struct {
	first uint32
	count int
	data  []byte
}

func (r *Reader) serialize() ([]byte, error) {
	if r.root == nil {
		return nil, errors.New("cfb: closed reader")
	}
	entries, ordered := flattenNodes(r.root)
	indexMap := make(map[*node]*serialEntry, len(entries))
	for _, entry := range entries {
		indexMap[entry.node] = entry
	}
	for _, entry := range entries {
		if entry.node.objType == objTypeStorage || entry.node.objType == objTypeRoot {
			children := append([]*node(nil), entry.node.children...)
			sort.Slice(children, func(i, j int) bool {
				return cfbNameLess(children[i].name, children[j].name)
			})
			root := buildTreeLinks(children, indexMap)
			if root != nil {
				entry.child = indexMap[root].index
			} else {
				entry.child = noStream
			}
		}
	}

	miniSectorCount := 0
	miniStream := &bytes.Buffer{}
	miniFAT := make([]uint32, 0)
	regularStreams := make([]*serialEntry, 0)
	for _, entry := range ordered {
		if entry.node.objType != objTypeStream {
			continue
		}
		entry.size = uint64(len(entry.node.data))
		if len(entry.node.data) > 0 && len(entry.node.data) < miniStreamCutoff {
			count := (len(entry.node.data) + 63) / 64
			entry.start = noStream
			entry.miniStart = uint32(miniSectorCount)
			for idx := 0; idx < count; idx++ {
				start := idx * 64
				end := start + 64
				if end > len(entry.node.data) {
					end = len(entry.node.data)
				}
				chunk := make([]byte, 64)
				copy(chunk, entry.node.data[start:end])
				miniStream.Write(chunk)
				if idx == count-1 {
					miniFAT = append(miniFAT, endOfChain)
				} else {
					miniFAT = append(miniFAT, uint32(miniSectorCount+1))
				}
				miniSectorCount++
			}
			if count == 0 {
				entry.miniStart = endOfChain
			}
			continue
		}
		regularStreams = append(regularStreams, entry)
	}

	rootEntry := entries[0]
	rootEntry.size = uint64(miniStream.Len())
	rootEntry.node.data = append([]byte(nil), miniStream.Bytes()...)
	rootEntry.regularLen = len(rootEntry.node.data)
	regularStreams = append([]*serialEntry{rootEntry}, regularStreams...)

	directorySectorCount := sectorCount(len(entries)*dirEntrySize, 512)
	miniFATBytes := serializeUint32Chain(miniFAT)
	miniFATSectorCount := sectorCount(len(miniFATBytes), 512)
	regularDataSectors := 0
	for _, entry := range regularStreams {
		entry.regularLen = len(entry.node.data)
		regularDataSectors += sectorCount(entry.regularLen, 512)
	}

	fatSectorCount := 0
	for {
		total := regularDataSectors + directorySectorCount + miniFATSectorCount + fatSectorCount
		needed := sectorCount(total*4, 512)
		if needed == fatSectorCount {
			break
		}
		fatSectorCount = needed
	}
	if fatSectorCount > 109 {
		return nil, errors.New("cfb: DIFAT serialization beyond 109 FAT sectors is not supported")
	}

	nextSector := uint32(0)
	fat := make([]uint32, regularDataSectors+directorySectorCount+miniFATSectorCount+fatSectorCount)
	for i := range fat {
		fat[i] = freeSect
	}

	regularPayloads := make([][]byte, 0, len(regularStreams)+2)
	for _, entry := range regularStreams {
		count := sectorCount(entry.regularLen, 512)
		if count == 0 {
			entry.start = endOfChain
			continue
		}
		entry.start = nextSector
		for idx := 0; idx < count; idx++ {
			sectorNum := nextSector + uint32(idx)
			if idx == count-1 {
				fat[sectorNum] = endOfChain
			} else {
				fat[sectorNum] = sectorNum + 1
			}
		}
		regularPayloads = append(regularPayloads, padToSector(entry.node.data, 512))
		nextSector += uint32(count)
	}

	dirStart := nextSector
	for idx := 0; idx < directorySectorCount; idx++ {
		sectorNum := dirStart + uint32(idx)
		if idx == directorySectorCount-1 {
			fat[sectorNum] = endOfChain
		} else {
			fat[sectorNum] = sectorNum + 1
		}
	}
	nextSector += uint32(directorySectorCount)

	miniFATStart := uint32(endOfChain)
	if miniFATSectorCount > 0 {
		miniFATStart = nextSector
		for idx := 0; idx < miniFATSectorCount; idx++ {
			sectorNum := miniFATStart + uint32(idx)
			if idx == miniFATSectorCount-1 {
				fat[sectorNum] = endOfChain
			} else {
				fat[sectorNum] = sectorNum + 1
			}
		}
		nextSector += uint32(miniFATSectorCount)
	}

	fatSectorStart := nextSector
	for idx := 0; idx < fatSectorCount; idx++ {
		fat[fatSectorStart+uint32(idx)] = fatSect
	}

	for _, entry := range ordered {
		if entry.node.objType == objTypeStream && entry.size < miniStreamCutoff && entry.size > 0 {
			entry.start = entry.miniStart
		}
	}

	dirBytes := serializeDirectoryEntries(entries)

	headerBytes := make([]byte, 512)
	copy(headerBytes[:8], []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1})
	binary.LittleEndian.PutUint16(headerBytes[24:26], 0x003E)
	binary.LittleEndian.PutUint16(headerBytes[26:28], 0x0003)
	binary.LittleEndian.PutUint16(headerBytes[28:30], 0xFFFE)
	binary.LittleEndian.PutUint16(headerBytes[30:32], 9)
	binary.LittleEndian.PutUint16(headerBytes[32:34], 6)
	binary.LittleEndian.PutUint32(headerBytes[40:44], 0)
	binary.LittleEndian.PutUint32(headerBytes[44:48], uint32(fatSectorCount))
	binary.LittleEndian.PutUint32(headerBytes[48:52], dirStart)
	binary.LittleEndian.PutUint32(headerBytes[56:60], miniStreamCutoff)
	binary.LittleEndian.PutUint32(headerBytes[60:64], miniFATStart)
	binary.LittleEndian.PutUint32(headerBytes[64:68], uint32(miniFATSectorCount))
	binary.LittleEndian.PutUint32(headerBytes[68:72], endOfChain)
	binary.LittleEndian.PutUint32(headerBytes[72:76], 0)
	for idx := 0; idx < 109; idx++ {
		value := uint32(freeSect)
		if idx < fatSectorCount {
			value = fatSectorStart + uint32(idx)
		}
		binary.LittleEndian.PutUint32(headerBytes[76+idx*4:80+idx*4], value)
	}

	output := &bytes.Buffer{}
	output.Write(headerBytes)
	for _, payload := range regularPayloads {
		output.Write(payload)
	}
	output.Write(padToSector(dirBytes, 512))
	if miniFATSectorCount > 0 {
		output.Write(padToSector(miniFATBytes, 512))
	}
	for idx := 0; idx < fatSectorCount; idx++ {
		sectorBuf := make([]byte, 512)
		base := idx * 128
		for entryIdx := 0; entryIdx < 128; entryIdx++ {
			value := uint32(freeSect)
			if base+entryIdx < len(fat) {
				value = fat[base+entryIdx]
			}
			binary.LittleEndian.PutUint32(sectorBuf[entryIdx*4:(entryIdx+1)*4], value)
		}
		output.Write(sectorBuf)
	}
	return output.Bytes(), nil
}

func flattenNodes(root *node) ([]*serialEntry, []*serialEntry) {
	entries := make([]*serialEntry, 0)
	ordered := make([]*serialEntry, 0)
	var walk func(current *node)
	walk = func(current *node) {
		entry := &serialEntry{
			node:  current,
			index: uint32(len(entries)),
			left:  noStream,
			right: noStream,
			child: noStream,
			start: endOfChain,
		}
		entries = append(entries, entry)
		ordered = append(ordered, entry)
		for _, child := range current.children {
			walk(child)
		}
	}
	walk(root)
	return entries, ordered
}

func buildTreeLinks(children []*node, indexMap map[*node]*serialEntry) *node {
	var build func(items []*node) *node
	build = func(items []*node) *node {
		if len(items) == 0 {
			return nil
		}
		mid := len(items) / 2
		current := items[mid]
		entry := indexMap[current]
		entry.left = noStream
		entry.right = noStream
		if left := build(items[:mid]); left != nil {
			entry.left = indexMap[left].index
		}
		if right := build(items[mid+1:]); right != nil {
			entry.right = indexMap[right].index
		}
		return current
	}
	return build(children)
}

func serializeDirectoryEntries(entries []*serialEntry) []byte {
	buf := &bytes.Buffer{}
	for _, entry := range entries {
		chunk := make([]byte, dirEntrySize)
		encodeUTF16Name(chunk[:64], entry.node.name)
		nameLength := 2
		if entry.node.name != "" {
			nameLength = (len(utf16.Encode([]rune(entry.node.name))) + 1) * 2
		}
		binary.LittleEndian.PutUint16(chunk[64:66], uint16(nameLength))
		chunk[66] = entry.node.objType
		chunk[67] = 1
		binary.LittleEndian.PutUint32(chunk[68:72], entry.left)
		binary.LittleEndian.PutUint32(chunk[72:76], entry.right)
		binary.LittleEndian.PutUint32(chunk[76:80], entry.child)
		copy(chunk[80:96], entry.node.clsid[:])
		binary.LittleEndian.PutUint32(chunk[96:100], entry.node.stateBits)
		binary.LittleEndian.PutUint64(chunk[100:108], entry.node.createdAt)
		binary.LittleEndian.PutUint64(chunk[108:116], entry.node.modifiedAt)
		binary.LittleEndian.PutUint32(chunk[116:120], entry.start)
		binary.LittleEndian.PutUint64(chunk[120:128], entry.size)
		buf.Write(chunk)
	}
	return buf.Bytes()
}

func serializeUint32Chain(values []uint32) []byte {
	buf := &bytes.Buffer{}
	for _, value := range values {
		_ = binary.Write(buf, binary.LittleEndian, value)
	}
	return buf.Bytes()
}

func sectorCount(byteLen int, sectorSize int) int {
	if byteLen == 0 {
		return 0
	}
	return (byteLen + sectorSize - 1) / sectorSize
}

func padToSector(data []byte, sectorSize int) []byte {
	if len(data) == 0 {
		return nil
	}
	if rem := len(data) % sectorSize; rem != 0 {
		padded := make([]byte, len(data)+(sectorSize-rem))
		copy(padded, data)
		return padded
	}
	return append([]byte(nil), data...)
}

func decodeUTF16Name(raw []byte) string {
	if len(raw) < 2 {
		return ""
	}
	raw = raw[:len(raw)-2]
	codeUnits := make([]uint16, 0, len(raw)/2)
	for offset := 0; offset+1 < len(raw); offset += 2 {
		value := binary.LittleEndian.Uint16(raw[offset : offset+2])
		if value == 0 {
			break
		}
		codeUnits = append(codeUnits, value)
	}
	return string(utf16.Decode(codeUnits))
}

func encodeUTF16Name(dst []byte, name string) {
	for i := range dst {
		dst[i] = 0
	}
	if name == "" {
		return
	}
	encoded := utf16.Encode([]rune(name))
	if len(encoded) > maxRegularName {
		encoded = encoded[:maxRegularName]
	}
	for idx, value := range encoded {
		binary.LittleEndian.PutUint16(dst[idx*2:(idx+1)*2], value)
	}
}

// cfbNameLess compares two CFB directory entry names per [MS-CFB] §2.6.4.
// Comparison order: 1) UTF-16 code unit length, 2) upper-cased UTF-16 code units.
func cfbNameLess(a, b string) bool {
	au := utf16.Encode([]rune(strings.ToUpper(a)))
	bu := utf16.Encode([]rune(strings.ToUpper(b)))
	if len(au) != len(bu) {
		return len(au) < len(bu)
	}
	for i := 0; i < len(au); i++ {
		if au[i] != bu[i] {
			return au[i] < bu[i]
		}
	}
	return false
}
