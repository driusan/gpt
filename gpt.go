// Package gpt provides a package for reading GPT partition tables from a
// block device in Go
//
// The cmd/gpt subdirectory contains a simple tool to read the existing
// GPT header/partition table and demonstrate how to use this package.
package gpt

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"unicode/utf16"
)

// Size of a block on the hard drive.
//
// TODO: This should be a variable and handle different sizes, but then the
// padding in GPT can't be built into the struct definition, and the
// LogicalBlock would need to be a slice instead of an array.
// BUG(driusan): Only 512 byte logical block sizes are currently supported.
const LogicalBlockSize uint64 = 512

// A single block on the hard drive.
type LogicalBlock [LogicalBlockSize]byte

// GPTHeader represents a GPT header. There should be two copies of this:
// one at block 1 (0-indexed, block 0 is for a protective MBR), and one at the
// address AltLBA. A GPTHeader should be exactly one LogicalBlock in size.
type GPTHeader struct {
	// Signature for this header.
	// Must be "EFI PART"
	Signature [8]byte

	// GPT version that this header is encoded with
	Revision uint32

	// must be >= 92 and <= logical block size
	HeaderSize uint32

	// A CRC32 check for this header. The CRC is encoded with this field
	HeaderCRC32 uint32

	// Must be zero (given a field name instead of _ so that it can
	// be verified.)
	Reserved uint32

	// The logical block address of this header
	// Should be "1" for the primary header, and AltLBA of the primary
	// header for the secondary header
	MyLBA uint64

	// The logical block address of the secondary/backup GPT header
	// (or the primary, if this header is the secondary)
	AltLBA uint64

	// The first block of the hard drive useable by GPT partitions
	FirstUseableLBA uint64
	// The last block of the hard drive useable by GPT partitions (there
	// must leave enough space for the secondary GPT header.)
	LastUseableLBA uint64

	// A unique GUID for this disk.
	Disk GUID

	// The block at which the GPT partition table starts
	PartitionEntryLBA uint64

	// The maximum number of partitions that can be stored in the GPT
	// partition table pointed to by PartitionEntryLBA. (May span multiple
	// blocks)
	MaxNumberPartitionEntries uint32

	// The size of a single GPT partition entry
	SizeOfPartitionEntry uint32

	// A CRC32 check of the partition entry array.
	PartitionEntryArrayCRC32 uint32

	// Zero padding to ensure that the GPTHeader takes up exactly 1 block.
	// Must be zero.
	Padding [LogicalBlockSize - 92]byte
}

// Verifies that the GPT header loaded from disk is valid.
//
// BUG(driusan): HeaderCRC32 not verified.
// BUG(driusan): PartitionEntryArrayCRC32 not verified
// BUG(driusan): Secondary header is not verified
func (g GPTHeader) Verify() error {
	if string(g.Signature[:]) != "EFI PART" {
		return fmt.Errorf("Invalid GPT Header \"%v\"", string(g.Signature[:]))
	}
	if g.Reserved != 0 {
		return fmt.Errorf("Invalid GPT Header. Reserved area not zero.")
	}
	for _, b := range g.Padding {
		if b != 0 {
			return fmt.Errorf("Invalid GPT Header. Header not zero padded.")
		}
	}

	if g.MyLBA != 1 || g.PartitionEntryLBA != 2 {
		return fmt.Errorf("TODO: Handle GPT Header or GPT Partition in non-standard location")
	}

	// TODO: Check HeaderCRC32
	// TODO: Check PartitionEntryCRC32
	return g.verifyAlt()
}

// Verifies the secondary GPT header. verifyAlt should *not* validate its own
// alternate header, as that would result in an infinite loop.
// BUG: This is not implemented at all.
func (g GPTHeader) verifyAlt() error {
	// TODO: Implement this.
	return nil
}

// Reads the GPT Partitions from the location pointed to from the GPT header
// hd should be a io.ReadSeeker (usually an os.File) pointing to the block
// device for the drive being read.
func (g GPTHeader) GetPartitions(hd io.ReadSeeker) ([]GPTPartitionEntry, error) {
	newOffset, err := hd.Seek(int64(LogicalBlockSize*(g.PartitionEntryLBA)), 0)
	if err != nil {
		return nil, err
	}
	if uint64(newOffset) != LogicalBlockSize*g.PartitionEntryLBA {
		return nil, fmt.Errorf("Could not find PartitionEntry table.")
	}
	partitions := make([]GPTPartitionEntry, 0, g.MaxNumberPartitionEntries)

	// We must load 1 logical block at a time, otherwise bad things happen
	// on some OSes
	partitionsLeft := g.MaxNumberPartitionEntries
	partitionsPerBlock := uint32(LogicalBlockSize / uint64(g.SizeOfPartitionEntry))
	if uint64(LogicalBlockSize)%uint64(g.SizeOfPartitionEntry) != 0 {
		return nil, fmt.Errorf("Partitions must fit entirely in a single block.")
	}
	for partitionsLeft > 0 {
		// Read one logical block to ensure that we don't get an I/O
		// error
		var hdBlock LogicalBlock
		_, err := io.ReadFull(hd, hdBlock[:])
		if err != nil {
			return nil, err
		}

		// then convert what we read into a reader, and read the
		// the partition entry from that, since the size of an entry
		// isn't the size of a block.
		partReader := bytes.NewReader(hdBlock[:])

		for i := uint32(0); i < partitionsPerBlock; i++ {
			p := GPTPartitionEntry{}
			err = binary.Read(partReader, binary.LittleEndian, &p)
			//err = binary.Read(partReader, binary.BigEndian, &p)
			if err != nil {
				return nil, err
			}

			// read the appropriate amount of padding to get to the next
			// entry and verify that it's all zeros
			if paddingSize := g.SizeOfPartitionEntry - 128; paddingSize > 0 {
				padding := make([]byte, paddingSize)

				// Make the padding non-zero, so that when we read it
				// we can be sure that it was read properly and not
				// just initialized to zero by Go.
				for i := uint32(0); i < paddingSize; i++ {
					padding[i] = 0xFF
				}

				n, err := io.ReadFull(partReader, padding)
				if err != nil {
					return nil, err
				}
				if uint32(n) != paddingSize {
					return nil, fmt.Errorf("Could not read appropriate number of zeros to pad partition entry")
				}
				for i := uint32(0); i < paddingSize; i++ {
					if padding[i] != 0 {
						return nil, fmt.Errorf("Invalid partition entry padding")
					}
				}
			}

			partitions = append(partitions, p)
			partitionsLeft--
		}
	}
	return partitions, nil
}

type GPTPartitionAttribute uint64

// Masks for bits in GPTPartitionAttribute.
// Bits 3-47 are reserved and must be zero.
// Bits 48-63 are reserved for GUID specific use and must be preserved by tools
// which modify the GPT header
const (
	GPTPartitionSystem = GPTPartitionAttribute(iota)
	GPTPartitionNoBlockIOProtocol
	GPTPartitionLegacyBIOSBootable
)

// Represents a single GPT partition.
// When reading a GPT partition from the disk, it's followed by
// len(sizeOfPartitionEntry)-128 zeros, which can't be encoded in this struct
// since SizeOfPartitionEntry is a variable encoded in the GPT Header
type GPTPartitionEntry struct {
	// The type of partition
	PartitionType GUID
	// A unique GUID for this instance of this partition
	UniqueParitition GUID

	// Starting and ending Logical Block Address of this partition.
	StartingLBA uint64
	EndingLBA   uint64

	// Attributes for this GPT partition
	Attributes GPTPartitionAttribute

	// A UTF16 encoded (yes, UEFI is really that stupid) string for the
	// name of this partition. May be empty. Use GetName() method to
	// get this value as a Go string type.
	PartitionName [36]uint16
}

// Returns the size of a GPT partition in number of logical blocks
func (e GPTPartitionEntry) Size() uint64 {
	return e.EndingLBA - e.StartingLBA
}

// Returns the name of the GPT partition.
func (e GPTPartitionEntry) GetName() string {
	for i := 0; i < len(e.PartitionName); i++ {
		c := e.PartitionName[i]
		if c == 0 {
			if i == 0 {
				return ""
			}
			return string(utf16.Decode(e.PartitionName[:i]))
		}
	}
	return string(utf16.Decode(e.PartitionName[:]))
}
