package md5

import (
	"encoding/hex"
	"errors"
	"crypto/md5"
	"io"
	"os"
	"strings"
)


// CalculateMD5
//	Calculate MD5Checksum for the transferred file.
//	Return back the byte array representation.
func CalculateMD5(file *os.File) ([]byte, error) {
	_, seekErr := file.Seek(0, 0)
	if seekErr != nil { return nil, seekErr }

	hash := md5.New()
	_, generateMd5Err := io.Copy(hash, file)
	if generateMd5Err != nil { return nil, generateMd5Err }

	return hash.Sum(nil), nil
}

// DeserializeMD5ToHex
//	Tranform byte representation of MD5Checksum to hex.
func DeserializeMD5ToHex(input []byte) (string, error) {
	if len(input) != 16 { return "", errors.New("input length for md5sum should be 16") }
	return hex.EncodeToString(input), nil
}

// SerializeMD5ToBytes
//	Transform a string representation of MD5Checksum to byte array.
//	For transferring on the wire.
func SerializeMD5ToBytes(input string) ([]byte, error) {
	md5 := strings.ReplaceAll(input, "\n", "")
	md5Bytes, decodeErr := hex.DecodeString(md5)
	if decodeErr != nil { return nil, decodeErr }

	return md5Bytes, nil
}

// ReadMD5FromFile
//	Read a MD5Checksum from a file.
//	The hex representation is then serialized to byte array.
func ReadMD5FromFile(md5FilePath string) ([]byte, error) {
	data, err := os.ReadFile(md5FilePath)
	if err != nil { return nil, err }

	md5Bytes, sErr := SerializeMD5ToBytes(string(data))
	if sErr != nil { return nil, sErr }
	if len(md5Bytes) != 16 { return nil, errors.New("md5 sum incorrect length") }
	
	return md5Bytes, nil
}