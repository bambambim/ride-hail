package uuid

import (
	"crypto/rand"
	"fmt"
)

// UUID is a 128-bit (16-byte) universally unique identifier.
type UUID [16]byte

// Nil is the zero value for a UUID.
var Nil = UUID{}

// NewV4 generates a new random UUID (version 4).
func NewV4() (UUID, error) {
	var u UUID
	// Read 16 random bytes
	if _, err := rand.Read(u[:]); err != nil {
		return Nil, err
	}

	// Set version (4) and variant (RFC4122) bits
	u[6] = (u[6] & 0x0f) | 0x40 // Version 4
	u[8] = (u[8] & 0x3f) | 0x80 // Variant is 10

	return u, nil
}

// MustNewV4 is a helper that panics if UUID generation fails.
func MustNewV4() UUID {
	u, err := NewV4()
	if err != nil {
		panic(fmt.Errorf("failed to generate UUID: %w", err))
	}
	return u
}

// String returns the UUID in the standard hexadecimal format (xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx).
func (u UUID) String() string {
	return fmt.Sprintf("%x-%x-%x-%x-%x",
		u[0:4], u[4:6], u[6:8], u[8:10], u[10:])
}

// MarshalJSON implements the json.Marshaler interface.
func (u UUID) MarshalJSON() ([]byte, error) {
	return []byte(`"` + u.String() + `"`), nil
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (u *UUID) UnmarshalJSON(b []byte) error {
	if len(b) < 2 || b[0] != '"' || b[len(b)-1] != '"' {
		return fmt.Errorf("invalid UUID JSON format")
	}
	s := string(b[1 : len(b)-1])
	if len(s) != 36 {
		return fmt.Errorf("UUID string must be 36 characters long")
	}

	// Define temporary holders for each segment of the UUID (in bytes: 4, 2, 2, 2, 6)
	// We use uint64 to safely hold the largest segment (6 bytes = 48 bits).
	var (
		d1 uint64
		d2 uint64
		d3 uint64
		d4 uint64
		d5 uint64
	)

	// 1. Scan the string into the temporary integers
	// Format: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
	// Segment sizes: 8-4-4-4-12 hex digits (32 total)
	n, err := fmt.Sscanf(s, "%8x-%4x-%4x-%4x-%12x",
		&d1, &d2, &d3, &d4, &d5)
	if err != nil {
		return fmt.Errorf("UUID parsing failed: %w", err)
	}
	if n != 5 {
		return fmt.Errorf("UUID format error: expected 5 segments, found %d", n)
	}

	// 2. Manually copy the bytes from the integers into the [16]byte array

	// Segment 1 (d1: 4 bytes)
	u[0] = byte(d1 >> 24)
	u[1] = byte(d1 >> 16)
	u[2] = byte(d1 >> 8)
	u[3] = byte(d1)

	// Segment 2 (d2: 2 bytes)
	u[4] = byte(d2 >> 8)
	u[5] = byte(d2)

	// Segment 3 (d3: 2 bytes)
	u[6] = byte(d3 >> 8)
	u[7] = byte(d3)

	// Segment 4 (d4: 2 bytes)
	u[8] = byte(d4 >> 8)
	u[9] = byte(d4)

	// Segment 5 (d5: 6 bytes)
	u[10] = byte(d5 >> 40)
	u[11] = byte(d5 >> 32)
	u[12] = byte(d5 >> 24)
	u[13] = byte(d5 >> 16)
	u[14] = byte(d5 >> 8)
	u[15] = byte(d5)

	return nil
}

// Value implements the database/sql/driver.Valuer interface for database insertion.
// pgx can generally handle [16]byte directly, but this provides a fallback/standard interface.
func (u UUID) Value() (interface{}, error) {
	return u[:], nil
}
