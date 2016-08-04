package gpt

import (
	"fmt"
)

// Represents a RFC 4122 GUID.
type GUID struct {
	TimeLow             uint32
	TimeMid             uint16
	TimeHighAndVersion  uint16
	ClockSeqAndReserved uint8
	ClockSeqLow         uint8
	Node                [6]byte
}

// The ZeroGUID is a nil GUID that can be used for comparison
var ZeroGUID GUID = GUID{0, 0, 0, 0, 0, [6]byte{0, 0, 0, 0, 0}}

// Converts a partition type GUID to a human readable string.
// BUG(driusan): Converting PartitionTypeGUID to a human readable string
// only supports partition types which are used on my computer, because
// I don't have time to transcribe every one on wikipedia.
func (g GUID) HumanString() string {
	switch guid := g.String(); guid {
	case "00000000-0000-0000-0000-0000000000000":
		return "Unused"
	case "C12A7328-F81F-11D2-BA4B-00A0C93EC93B":
		return "EFI System Partition"
	case "0657FD6D-A4AB-43C4-84E5-0933C84B4F4F":
		return "Linux Swap"
	case "9D94CE7C-1CA5-11DC-8817-01301BB8A9F5":
		return "DragonFly UFS1"
	case "C91818F9-8025-47AF-89D2-F030D7000C2C":
		return "Plan 9"
	case "824CC7A0-36A8-11E3-890A-952519AD3F61":
		return "OpenBSD"
	case "0FC63DAF-8483-4772-8E79-3D69D8477DE4":
		return "Linux"
	default:
		return guid
	}
}

// Converts a GUID to a standard string representation
func (g GUID) String() string {
	return fmt.Sprintf("%0.8X-%0.4X-%0.4X-%0.2X%0.2X-%0.16X", g.TimeLow, g.TimeMid, g.TimeHighAndVersion, g.ClockSeqAndReserved, g.ClockSeqLow, g.Node)
}
