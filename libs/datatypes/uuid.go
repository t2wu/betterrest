package datatypes

import (
	"database/sql/driver"
	"fmt"
	"log"
	"strings"

	uuid "github.com/satori/go.uuid"
)

// UUID string
type UUID struct {
	UUID uuid.UUID
}

// NewUUID generates a UUID that's partly V1 and partly V4
// Like V4, but use timestamp as part of V1 to increase locality (performance)
func NewUUID() *UUID {
	toks1 := strings.SplitN(uuid.NewV1().String(), "-", 2)
	toks2 := strings.SplitN(uuid.NewV4().String(), "-", 2)
	u, err := uuid.FromString(toks1[0] + "-" + toks2[1])
	if err != nil {
		log.Println("err:", err)
		panic("NewUUID() error")
	}

	return &UUID{UUID: u}
}

// NewUUIDFromString creates UUID from string
func NewUUIDFromString(s string) (u *UUID, err error) {
	u = &UUID{}
	u.UUID, err = uuid.FromString(s)
	return u, err
}

// Value satisfies the Valuer interace and is responsible for writing data to the database
func (u *UUID) String() string {
	return u.UUID.String()
}

// For Postgresql, Value()  can be string or uint8
// But when Scan, it is string. So I make it consistent to uint8

// Value satisfies the Valuer interace and is responsible for writing data to the database
func (u *UUID) Value() (driver.Value, error) {
	if u == nil {
		return nil, nil
	}

	return []uint8(u.UUID.String()), nil
}

// Scan satisfies the Scanner interace and is responsible for reading data from the database
func (u *UUID) Scan(src interface{}) error {
	if src == nil {
		return nil
	}

	// Huh? Sometimes []uint8 sometimes string?
	var err error
	s, ok := src.([]uint8)
	if ok {
		u.UUID, err = uuid.FromString(string(s))
		return err
	}

	s2, ok := src.(string)
	if ok {
		u.UUID, err = uuid.FromString(s2)
		return err
	}

	return fmt.Errorf("did not scan: expected []uint8 but was %T", src)
}

// UnmarshalJSON unmarshalls it from a string of millisecond
func (u *UUID) UnmarshalJSON(b []byte) (err error) {
	if len(b) > 2 && b[0] == '"' && b[len(b)-1] == '"' {
		b = b[1 : len(b)-1]
	}

	uu, err := uuid.FromString(string(b)) // FromBinary doesn't work, does it ha anything to do with endianness?
	u.UUID = uu
	return err
}

// MarshalJSON customizes unmarshalling from JSON array
func (u *UUID) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("\"%s\"", u.UUID.String())), nil
}
