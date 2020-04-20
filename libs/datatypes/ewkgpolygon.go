package datatypes

import (
	"database/sql/driver"
	"encoding/binary"
	"encoding/json"
	"fmt"

	"github.com/twpayne/go-geom"
	"github.com/twpayne/go-geom/encoding/wkb"
)

// EWKBPolygon encapsulate Polygon and handles value and scanners to work with Gorm
type EWKBPolygon struct {
	Polygon wkb.Polygon
}

// Value satisfies the Valuer interace and is responsible for writing data to the database
func (m *EWKBPolygon) Value() (driver.Value, error) {

	value, err := m.Polygon.Value()
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
func (m *EWKBPolygon) Scan(src interface{}) error {
	if src == nil {
		return nil
	}

	mysqlEncoding, ok := src.([]byte)
	if !ok {
		return fmt.Errorf("did not scan: expected []byte but was %T", src)
	}

	srid := binary.LittleEndian.Uint32(mysqlEncoding[0:4]) // uint32
	err := m.Polygon.Scan(mysqlEncoding[4:])
	m.Polygon.SetSRID(int(srid))

	return err
}

// UnmarshalJSON json satisfies the JSON library
// https://en.wikipedia.org/wiki/GeoJSON
// {
//     "type": "Polygon",
//     "coordinates": [
//         [[30, 10], [40, 40], [20, 40], [10, 20], [30, 10]]
//     ]
// }
func (m *EWKBPolygon) UnmarshalJSON(b []byte) (err error) {
	// loc := []float64{0, 0}
	area := struct {
		Type        string
		Coordinates [][]geom.Coord // type Coord []float64
	}{
		"Polygon",
		[][]geom.Coord{{{0, 0}}},
	}
	if err := json.Unmarshal(b, &area); err != nil {
		return err
	}

	poly, err := geom.NewPolygon(geom.XY).SetCoords(area.Coordinates)
	m.Polygon = wkb.Polygon{poly}

	return err // if err is nil, all good
}

// MarshalJSON customizes unmarshalling from JSON array (e.g. [10, 20])
func (m *EWKBPolygon) MarshalJSON() ([]byte, error) {
	p := m.Polygon
	coords := p.Coords()[0] // [][]Coord
	if len(coords) > 0 {
		s := fmt.Sprintf("[[%f, %f]", coords[0][0], coords[0][1])
		for _, v := range coords[1:] {
			s = s + fmt.Sprintf(",[%f, %f]", v[0], v[1])
		}
		s = s + "]"
		return []byte(fmt.Sprintf("{\"type\": \"Polygon\", \"coordinates\": [%s] }", s)), nil
	}

	return []byte("{\"type\": \"Polygon\", \"coordinates\": [] }"), nil
}
