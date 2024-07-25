package types

import (
	"fmt"
	"io"
)

func writePacked(w io.Writer, data []byte) error {
	for len(data)%4 != 0 {
		data = append(data, 0x00)
	}
	if _, err := w.Write(data); err != nil {
		return err
	}
	return nil
}

func readPacked(r io.Reader, length uint) ([]byte, error) {
	paddingLength := uint(0)
	if length%4 != 0 {
		paddingLength = 4 - length%4
	}

	data := make([]byte, length+paddingLength)
	if _, err := r.Read(data); err != nil {
		return nil, err
	}
	return data[0:length], nil
}

func bytesToHexString(data []byte) string {
	str := ""
	for i, d := range data {
		str += fmt.Sprintf("%.2x", d)
		if i != len(data)-1 {
			str += " "
		}
	}
	return str
}

func bytesToByteString(data []byte) string {
	str := ""
	for i, d := range data {
		str += fmt.Sprintf("%d", d)
		if i != len(data)-1 {
			str += " "
		}
	}
	return str
}
