package serialize

import "errors"
import "encoding/binary"
import "math/big"


//============================================= Serialize


// Below are utility functions for serializing and deserializing primitives into and from byte arrays


func SerializeBigInt(in *big.Int, totalBytes int) []byte {
	buf := make([]byte, totalBytes)
	return in.FillBytes(buf)
}

func DeserializeBigInt(data []byte, totalBytes int) (*big.Int, error) {
	if len(data) != totalBytes { return nil, errors.New("invalid data length for total bytes provided") }
	
	num := new(big.Int)
	num.SetBytes(data)
	return num, nil
}

func SerializeUint64(in uint64) []byte {
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, in)
	return buf
}

func DeserializeUint64(data []byte) (uint64, error) {
	if len(data) != 8 { return uint64(0), errors.New("invalid data length for byte slice to uint64") }
	return binary.LittleEndian.Uint64(data), nil
}

func SerializeUint32(in uint32) []byte {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, in)
	return buf
}

func DeserializeUint32(data []byte) (uint32, error) {
	if len(data) != 4 { return uint32(0), errors.New("invalid data length for byte slice to uint32") }
	return binary.LittleEndian.Uint32(data), nil
}

func SerializeUint16(in uint16) []byte {
	buf := make([]byte, 2)
	binary.LittleEndian.PutUint16(buf, in)
	return buf
}

func DeserializeUint16(data []byte) (uint16, error) {
	if len(data) != 2 { return uint16(0), errors.New("invalid data length for byte slice to uint16") }
	return binary.LittleEndian.Uint16(data), nil
}

func SerializeBool(isTrue bool) byte {
	if isTrue { 
		return 0x01 
	} else { return 0x00 }
}

func DeserializeBool(data byte) bool {
	if data == 0x01 {
		return true
	} else { return false }
}