package datatypes

import (
	"database/sql/driver"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/twpayne/go-geom"
	"github.com/twpayne/go-geom/encoding/wkb"
	// Might be able to use ewkb library with PostGis if I could figure out how to work with go-geom
	// Right now I'm not storing SRID in there
)

// https://stackoverflow.com/questions/60520863/working-with-spatial-data-with-gorm-and-mysql/

// NewEWKBPoint creates a new EWKBPoint
func NewEWKBPoint(coords []float64) *EWKBPoint {
	p := &EWKBPoint{}
	p.Point = wkb.Point{Point: geom.NewPoint(geom.XY).MustSetCoords(coords).SetSRID(0)}
	return p
}

// EWKBPoint encapsulate Point and handles value and scanners to work with Gorm
// Fetch by .Point.X(), .Point.Y()
type EWKBPoint struct {
	Point wkb.Point
}

// Value satisfies the Valuer interace and is responsible for writing data to the database
func (m *EWKBPoint) Value() (driver.Value, error) {
	// When updating a pegassoc but not give any value, it runs into this
	if m == nil {
		return "SRID=0;POINT(0 0)", nil
	}

	pt := m.Point
	pos := pt.Coords()

	// How is it stored? I can't tell.

	// return fmt.Sprintf("SRID=32632;POINT(%f %f)", pos[0], pos[1]), nil
	return fmt.Sprintf("SRID=0;POINT(%f %f)", pos[0], pos[1]), nil
}

// Scan satisfies the Scanner interace and is responsible for reading data from the database
func (m *EWKBPoint) Scan(src interface{}) error {
	if src == nil {
		return nil
	}

	// mysqlEncoding, err := hex.DecodeString(string(src.([]byte)))
	// var wkbByteOrder byte
	// reader := bytes.NewReader(mysqlEncoding)
	// // Read as Little Endian to attempt to determine byte order
	// if err := binary.Read(reader, binary.LittleEndian, &wkbByteOrder); err != nil {
	// 	return err
	// }

	hexdata, err := hex.DecodeString(string(src.([]byte)))
	if err != nil {
		return err
	}

	m.Point.Scan(hexdata)

	// Determine the geometery type
	// var byteOrder binary.ByteOrder
	// var wkbType uint32
	// if err := binary.Read(reader, byteOrder, &wkbType); err != nil {
	// 	return err
	// }

	// hexdata, err := hex.DecodeString(string(src.([]byte)))

	// srid := binary.LittleEndian.Uint32(hexdata[1:5]) // uint32
	// err = m.Point.Scan(hexdata[5:])
	// m.Point.SetSRID(int(srid))

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

	// I want to avoid having to use custom New() before unmarshalling
	// because that makes generic handling across all models impossible
	// And this is what I come up with. (Weird with pointer to struct
	// embedding)
	m.Point = wkb.Point{Point: geom.NewPoint(geom.XY).MustSetCoords([]float64{0, 0}).SetSRID(0)}

	pt := m.Point
	_, err = pt.SetCoords(geom.Coord{loc.Coordinates[0], loc.Coordinates[1]})
	return err // if err is nil, all good
}

// MarshalJSON customizes unmarshalling from JSON array (e.g. [10, 20])
func (m *EWKBPoint) MarshalJSON() ([]byte, error) {
	pt := m.Point
	pos := pt.Coords()
	if len(pos) == 2 {
		// lon first, lat second
		return []byte(fmt.Sprintf("{\"type\": \"Point\", \"coordinates\": [%f, %f] }", pos[0], pos[1])), nil
	}

	return []byte("[]"), nil
}
