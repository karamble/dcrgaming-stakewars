package replay

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

func decode(data []byte, r *Recording) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(r); err != nil {
		return err
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return errors.New("trailing replay data")
	}
	return nil
}
