package spatialdata

import (
	"database/sql/driver"
	"encoding/binary"
	"encoding/json"
	"fmt"

	"github.com/twpayne/go-geom"
	"github.com/twpayne/go-geom/encoding/wkb"
)

// https://stackoverflow.com/questions/60520863/working-with-spatial-data-with-gorm-and-mysql/

// EWKBPoint encapsulate Point and handles value and scanners to work with Gorm
type EWKBPoint struct {
	Point wkb.Point
}

// Value satisfies the Valuer interace and is responsible for writing data to the database
func (m *EWKBPoint) Value() (driver.Value, error) {
	value, err := m.Point.Value()
	if err != nil {
		return nil, err
	}

	buf, ok := value.([]byte)
	if !ok {
		return nil, fmt.Errorf("did not convert value: expected []byte, but was %T", value)
	}

	mysqlEncoding := make([]byte, 4)
	binary.LittleEndian.PutUint32(mysqlEncoding, 0)
	mysqlEncoding = append(mysqlEncoding, buf...)

	return mysqlEncoding, err
}

// Scan satisfies the Scanner interace and is responsible for reading data from the database
func (m *EWKBPoint) Scan(src interface{}) error {
	if src == nil {
		return nil
	}

	mysqlEncoding, ok := src.([]byte)
	if !ok {
		return fmt.Errorf("did not scan: expected []byte but was %T", src)
	}

	srid := binary.LittleEndian.Uint32(mysqlEncoding[0:4]) // uint32
	err := m.Point.Scan(mysqlEncoding[4:])
	m.Point.SetSRID(int(srid))

	return err
}

// UnmarshalJSON json satisfies the JSON library
// {
//     "type": "Point",
//     "coordinates": [30, 10]
// }
func (m *EWKBPoint) UnmarshalJSON(b []byte) (err error) {
	// loc := []float64{0, 0}
	loc := struct {
		Type        string
		Coordinates []float64
	}{
		"Point",
		[]float64{0, 0},
	}
	if err := json.Unmarshal(b, &loc); err != nil {
		return err
	}

	pt := m.Point
	_, err = pt.SetCoords(geom.Coord{loc.Coordinates[0], loc.Coordinates[1]})
	return err // if err is nil, all good
}

// MarshalJSON customizes unmarshalling from JSON array (e.g. [10, 20])
func (m *EWKBPoint) MarshalJSON() ([]byte, error) {
	pt := m.Point
	pos := pt.Coords()
	if len(pos) == 2 {
		return []byte(fmt.Sprintf("{\"type\": \"Point\", \"coordinates\": [%f, %f] }", pos[0], pos[1])), nil
	}

	return []byte("[]"), nil
}
